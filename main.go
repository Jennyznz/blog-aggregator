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

	"blog-aggregator/internal/config"
	"blog-aggregator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
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
	cmds.register("addfeed", handlerAddFeed)
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", handlerFollow)
	cmds.register("following", handlerFollowing)

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
		fmt.Errorf("command not found")
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
	result, err := fetchFeed(context.Background(), "https://www.wagslane.dev/index.xml")
	if err != nil {
		return err
	}

	fmt.Println(result)
	return nil
}

func handlerAddFeed(s *state, cmd command) error {
	if len(cmd.arguments) != 2 {
		fmt.Errorf("addfeed command expects 2 arguments")
	}

	// Extract feed name and url
	feedName := cmd.arguments[0]
	feedUrl := cmd.arguments[1]

	// Get current user
	currentUsername := s.cfg.CurrentUserName
	currentUser, err := s.db.GetUser(context.Background(), currentUsername)
	if err != nil {
		return err
	}

	// Create feed and connect to current user's ID
	feed, err := s.db.CreateFeed(context.Background(), database.CreateFeedParams{
		ID: uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name: feedName,
		Url: feedUrl,
		UserID: currentUser.ID,
	})
	if err != nil {
		return err
	}
	
	fmt.Println(feed)
	return nil
}

func handlerFeeds(s *state, cmd command) error {
	if len(cmd.arguments) > 0 {
		fmt.Errorf("feed command expects 0 arguments")
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

func handlerFollow(s *state, cmd command) error {
	if len(cmd.arguments) != 1 {
		return fmt.Errorf("follow command expects 1 argument")
	}

	// Get current user's name and ID
	currentUsername := s.cfg.CurrentUserName
	user, err := s.db.GetUser(context.Background(), currentUsername)
	if err != nil {
		return err
	}
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
	if err != nil {
		return err
	}

	fmt.Println(feed.Name, currentUsername)

	return nil
}

func handlerFollowing(s *state, cmd command) error {
	if len(cmd.arguments) > 0 {
		return fmt.Errorf("following command expects 0 arguments")
	}

	// Get ID of current user
	user, err := s.db.GetUser(context.Background(), s.cfg.CurrentUserName)
	if err != nil {
		return err
	}

	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), user.ID)
	if err != nil {
		return err
	}

	for _, feed := range feedFollows {
		fmt.Println(feed.FeedName)
	}

	return nil
}