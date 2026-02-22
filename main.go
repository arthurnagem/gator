package main

import (
	"context"
	"fmt"
	"os"
	"time"
	"database/sql"
    _ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/arthurnagem/gator/internal/config"
	"github.com/arthurnagem/gator/internal/database"
)
type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	name string
	args []string
}

type commands struct {
	handlers map[string]func(*state, command) error
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.handlers[name] = f
}

func (c *commands) run(s *state, cmd command) error {
	handler, exists := c.handlers[cmd.name]
	if !exists {
		return fmt.Errorf("unknown command: %s", cmd.name)
	}
	return handler(s, cmd)
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]

	_, err := s.db.GetUser(context.Background(), username)
	if err != nil {
		fmt.Println("user does not exist")
		os.Exit(1)
	}

	if err := s.cfg.SetUser(username); err != nil {
		return err
	}

	fmt.Printf("User set to %s\n", username)
	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) < 1 {
		return fmt.Errorf("username is required")
	}

	name := cmd.args[0]

	user, err := s.db.CreateUser(
		context.Background(),
		database.CreateUserParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Name:      name,
		},
	)

	if err != nil {
		fmt.Println("user already exists")
		os.Exit(1)
	}

	if err := s.cfg.SetUser(name); err != nil {
		return err
	}

	fmt.Println("User created successfully:")
	fmt.Println(user)

	return nil
}


func main() {
	if len(os.Args) < 2 {
		fmt.Println("not enough arguments")
		os.Exit(1)
	}

	cfg, err := config.Read()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.DBURL)
	if err != nil {
	fmt.Println(err)
	os.Exit(1)
	}

	dbQueries := database.New(db)

	s := &state{
		db:  dbQueries,
		cfg: &cfg,
	}

	cmds := &commands{
		handlers: make(map[string]func(*state, command) error),
	}

	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)


	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	if err := cmds.run(s, cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
