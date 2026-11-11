package main

import (
	"fmt"
	"os"
	"database/sql"
	"time"
	"context"
	"encoding/xml"
	"net/http"
	"html"
	"io"
	"errors"
	"strconv"

	"blog-aggregator/internal/config"
	"blog-aggregator/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func main() {
	configStruct, err := config.Read()
	if err != nil {
		return
	}
	// load database URL
	db, err := sql.Open("postgres", configStruct.DBURL)
	// create database queries
	dbQueries := database.New(db)

	// store config struct and db queries
	s := state { 
		cfg: &configStruct,
		db: dbQueries,
	}
	cmds := commands { commandsMap: make(map[string]func(*state, command) error) }	// initialize map of handler functions within the new instance

	// register handlers
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

	// get command line args
	args := os.Args
	if len(args) < 2 {
		fmt.Println("Please enter at least 2 arguments")
		os.Exit(1)
	}

	// split command line argments into name and additional arguments
	n := args[1]	// first argument is automatically the program name
	others := args[2:]
	cmd := command {
		name: n,
		arguments: others,
	}

	// run command and print errors if applicable
	err = cmds.run(&s, cmd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type state struct {
	cfg *config.Config
	db *database.Queries
}

type command struct {
	name string
	arguments []string
}

// maps commands to functions
type commands struct {
	commandsMap map[string]func(*state, command) error
}

func handlerLogin(s *state, cmd command) error {
	// login expects one argument: username
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("login command expects one argument")
	}

	username := cmd.arguments[0]
	
	// Check if username exists
	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		return fmt.Errorf("user does not exist")
	}

	// Login
	s.cfg.SetUser(username)

	fmt.Println("username has been set")
	return nil
}

func handlerRegister(s *state, cmd command) error {
	// register expects one argument: username
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("register command expects one argument.")
	}

	username := cmd.arguments[0]

	// Check if user already exists
	_, err := s.db.GetUser(context.Background(), username)
	if err == nil {
		return fmt.Errorf("this user already exists")
	}

	_, err = s.db.CreateUser(context.Background(), database.CreateUserParams{
		ID: uuid.New(), 
		CreatedAt: time.Now(), 
		UpdatedAt: time.Now(), 
		Name: username,
	})
	if err != nil {
		return err
	}
	
	// Login
	err = s.cfg.SetUser(username)
	if err != nil {
		return err
	}

	fmt.Println("user created")	
	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.arguments) > 0 {
		return fmt.Errorf("reset command expects zero arguments")
	}

	err := s.db.Reset(context.Background())
	if err != nil {
		return err
	}

	return nil
}

func handlerGetUsers(s *state, cmd command) error {
	if len(cmd.arguments) > 0 {
		return fmt.Errorf("users command expects zero arguments")
	}

	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		return err
	}

	for _, user := range users {
		if user.Name == s.cfg.CurrentUserName {
			fmt.Printf("* %s (current) \n", user.Name)
		} else {
			fmt.Printf("* %s \n", user.Name)
		}
	}

	return nil
}

func (c *commands) run(s *state, cmd command) error {
	function, ok := c.commandsMap[cmd.name]
	if ok {
		err := function(s, cmd)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("command not found")
	}

	return nil
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.commandsMap[name] = f
}

type RSSFeed struct {
	Channel struct {
		Title string `xml:"title"`
		Link string `xml:"link"`
		Description string `xml:"description"`
		Item []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title string `xml:"title"`
	Link string `xml:"link"`
	Description string `xml:"description"`
	PubDate string `xml:"pubDate"`
}

func fetchFeed (ctx context.Context, feedURL string) (*RSSFeed, error) {

	// Construct request
	req, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error building request")
	}
	req.Header.Set("User-Agent", "gator")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error sending request")
	}

	// Process response
	content, err := io.ReadAll(resp.Body)
	if err != nil{ 
		return &RSSFeed{}, fmt.Errorf("error reading response stream")
	}
	result := &RSSFeed{}
	err = xml.Unmarshal(content, result)
	if err != nil {
		return &RSSFeed{}, fmt.Errorf("error unmarshalling XML")
	}

	// Decode escaped HTML entities
	result.Channel.Title = html.UnescapeString(result.Channel.Title)
	result.Channel.Description = html.UnescapeString(result.Channel.Description)
	items := result.Channel.Item
	for i, _ := range(items) {
		items[i].Title = html.UnescapeString(items[i].Title)
		items[i].Description = html.UnescapeString(items[i].Description)
	}
 
	return result, nil
}

func handlerAgg (s *state, cmd command) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("agg command expects 1 argument")
	}

	// Extract and parse the provided  time between requests
	duration := cmd.arguments[0]
	fmt.Printf("Collecting feeds every %s\n", duration)
	parsedDuration, err := time.ParseDuration(duration)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(parsedDuration)
	for ; ; <-ticker.C {
		err := aggregation(s)
		if err != nil {
			fmt.Println("aggregation error:", err)
		}
	}

	result, err := fetchFeed(context.Background(), "https://news.ycombinator.com/rss")
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func aggregation(s *state) error {
	// Get next feed and mark as fetched
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())
	if err != nil {
		return err
	}

	err = s.db.MarkFeedFetched(context.Background(), nextFeed.ID)
	if err != nil {
		return err
	}

	// Fetch next feed 
	result, err := fetchFeed(context.Background(), nextFeed.Url)
	if err != nil {
		return err
	}

	// Get feed info from database
	feed, err := s.db.GetFeedFromUrl(context.Background(), nextFeed.Url)
	if err != nil {
		return err
	}

	// Save posts on feed to database
	for _, item := range result.Channel.Item {
		// Handle different formats for published_at. Fall back to time.Now() if parsing fails
		publishedAt, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			publishedAt = time.Now()
		}
	
		post, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID: uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Title: item.Title,
			Url: item.Link,
			Description: item.Description,
			PublishedAt: publishedAt,
			FeedID: feed.ID,
		})
		if err != nil {
			// Ignore 'URL exists' errors 
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unwrap err into a *pq.Error and check against PGSQL code for unique violation
				// ignore
				continue
			} else {
				// log all other errors
				return err
			}
		}

		fmt.Println("Post saved to database")
		fmt.Println(post)
	}
	return nil
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 2 {
		return fmt.Errorf("addfeed command expects 2 arguments")
	}

	// Extract feed name and url
	feedName := cmd.arguments[0]
	feedUrl := cmd.arguments[1]

	// Create feed and connect to current user's ID
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name: feedName,
		Url: feedUrl,
		UserID: user.ID,
	})
	if err != nil {
		return err
	}

	// Add entry to feed_follows database
	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return err
	}
	
	fmt.Println(feed)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	if len(cmd.arguments) > 0 {
		return fmt.Errorf("feeds command expects 0 arguments")
	}

	feeds, err := s.db.GetFeeds(context.Background())
	if err != nil {
		return err
	}

	for idx, _ := range feeds {
		// Get name of user who created feed
		user, err := s.db.GetUserFromId(context.Background(), feeds[idx].UserID)
		if err != nil {
			return err
		}

		fmt.Println(feeds[idx].Name, feeds[idx].Url, user.Name)
	}

	return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("follow command expects 1 argument")
	}

	// Get current user's ID
	currentUserID := user.ID

	// Get id of feed at provided URL
	url := cmd.arguments[0]
	feed, err := s.db.GetFeedFromUrl(context.Background(), url)
	if err != nil {
		return err
	}
	feedID := feed.ID

	// Create a new feed follow record for current user
	_, err = s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID: currentUserID,
		FeedID: feedID,
	})
	// Will return an error if user is already following that feed
	
	if err != nil {
		return err
	}

	fmt.Println(feed.Name, user.Name)

	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) > 0 {
		return fmt.Errorf("following command expects 0 arguments")
	}

	// Fetch feed follows for current user
	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	for _, feed := range feedFollows {
		fmt.Println(feed.FeedName)
	}

	return nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(s *state, cmd command) error {
	return func(s *state, cmd command) error {
		// Get current user
		user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
		if err != nil {
			return err
		}

		// Call function with user
		err = handler(s, cmd, user)
		if err != nil {
			return err
		}

		return nil
	}
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("unfollow command expects 1 argument")
	}

	// Get feedID of provided URL
	feed, err := s.db.GetFeedFromUrl(context.Background(), cmd.arguments[0])
	if err != nil {
		return err
	}

	err = s.db.FeedUnfollow(context.Background(), database.FeedUnfollowParams{
		user.ID, 
		feed.ID,
	})
	if err != nil {
		return err
	}

	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	if len(cmd.arguments) > 1 {
		return fmt.Errorf("browse command expects 0 or 1 arguments")
	}

	// Get post limit, or default to 2
	limit := "2"
	if len(cmd.arguments) == 1 {
		limit = cmd.arguments[0]
	}

	// Convert limit to int32
	intLimit, err := strconv.Atoi(limit)
	if err != nil {
		return err
	}
	int32Limit := int32(intLimit)

	// Get current user's posts from database
	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
							UserID: user.ID,
							Limit: int32Limit,
						})
	if err != nil {
		return err
	}

	// Print posts to console
	for _, post := range posts {
		fmt.Println(post)
	}

	return nil
}