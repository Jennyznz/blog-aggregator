package main

import (
	"blog-aggregator/internal/config"
	"fmt"
	"os"
)

func main() {
	configFile, err := config.Read()
	if err != nil {
		return
	}
	s := state { cfg: &configFile }
	cmds := commands { commandsMap: make(map[string]func(*state, command) error) }	// initialize map of handler functions within the new instance

	// register login handler
	cmds.register("login", handlerLogin)

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

	fmt.Println(config.Read())
}

type state struct {
	cfg *config.Config
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
	if len(cmd.arguments) == 0 {
		return fmt.Errorf("login command expects one argument")
	}

	username := cmd.arguments[0]
	s.cfg.SetUser(username)

	fmt.Println("username has been set")
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