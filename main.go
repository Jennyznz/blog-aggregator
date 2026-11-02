package main

import (
	"fmt"
	"os"
	"database/sql"
	"time"
	"context"

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