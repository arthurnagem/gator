package main

import (
	"context"
	"errors"
	"strconv"
    "strings"
	"fmt"
	"os"
	"time"
	"database/sql"
    _ "github.com/lib/pq"
	"github.com/google/uuid"
	"github.com/arthurnagem/gator/internal/config"
	"github.com/arthurnagem/gator/internal/database"
	"github.com/arthurnagem/gator/internal/rss"
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

func handlerReset(s *state, cmd command) error {
	err := s.db.ResetUsers(context.Background())
	if err != nil {
		fmt.Println("error resetting database:", err)
		os.Exit(1)
	}

	fmt.Println("database reset successfully")
	return nil
}

func handlerUsers(s *state, cmd command) error {
	users, err := s.db.GetUsers(context.Background())
	if err != nil {
		fmt.Println("error fetching users:", err)
		os.Exit(1)
	}

	for _, user := range users {
		if user.Name == s.cfg.CurrentUserName {
			fmt.Printf("* %s (current)\n", user.Name)
		} else {
			fmt.Printf("* %s\n", user.Name)
		}
	}

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("usage: agg <time_between_reqs>")
	}

	durationStr := cmd.args[0]

	timeBetweenRequests, err := time.ParseDuration(durationStr)
	if err != nil {
		return err
	}

	fmt.Println("Collecting feeds every", timeBetweenRequests)

	ticker := time.NewTicker(timeBetweenRequests)
	defer ticker.Stop()

	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, user database.User) error {
	if len(cmd.args) < 2 {
		return fmt.Errorf("usage: addfeed <name> <url>")
	}

	name := cmd.args[0]
	url := cmd.args[1]

	ctx := context.Background()

	feed, err := s.db.CreateFeed(ctx, database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
		Url:       url,
		UserID:    user.ID,
	})
	if err != nil {
		fmt.Println("error creating feed:", err)
		os.Exit(1)
	}

	_, err = s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:        uuid.New(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		fmt.Println("error creating feed follow:", err)
		os.Exit(1)
	}

	fmt.Println("Feed created successfully:")
	fmt.Println(feed)

	return nil
}

func handlerFeeds(s *state, cmd command) error {
    if len(cmd.args) != 0 {
        fmt.Println("usage: feeds")
        os.Exit(1)
    }

    feeds, err := s.db.GetFeeds(context.Background())
    if err != nil {
        return err
    }

    for _, feed := range feeds {
        fmt.Printf("* %s\n", feed.Name)
        fmt.Printf("  - URL: %s\n", feed.Url)
        fmt.Printf("  - Created by: %s\n\n", feed.UserName)
    }

    return nil
}

func handlerFollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		fmt.Println("usage: follow <url>")
		os.Exit(1)
	}

	url := cmd.args[0]

	cfg, err := config.Read()
	if err != nil {
		return err
	}

	if cfg.CurrentUserName == "" {
		return errors.New("no current user")
	}

	ctx := context.Background()

	feed, err := s.db.GetFeedByURL(ctx, url)
	if err != nil {
		return err
	}

	follow, err := s.db.CreateFeedFollow(ctx, database.CreateFeedFollowParams{
		ID:     uuid.New(),
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s is now following %s\n", follow.UserName, follow.FeedName)
	return nil
}

func handlerFollowing(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 0 {
		fmt.Println("usage: following")
		os.Exit(1)
	}

	ctx := context.Background()

	follows, err := s.db.GetFeedFollowsForUser(ctx, user.ID)
	if err != nil {
		return err
	}

	for _, f := range follows {
		fmt.Printf("* %s\n", f.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, user database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("usage: unfollow <feed_url>")
	}

	feedURL := cmd.args[0]
	ctx := context.Background()

	feed, err := s.db.GetFeedByURL(ctx, feedURL)
	if err != nil {
		return fmt.Errorf("feed not found")
	}

	err = s.db.DeleteFeedFollow(ctx, database.DeleteFeedFollowParams{
	UserID: user.ID,
	FeedID: feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Println("Unfollowed:", feed.Name)
	return nil
}

func handlerBrowse(s *state, cmd command, user database.User) error {
	limit := 2

	if len(cmd.args) == 1 {
		parsed, err := strconv.Atoi(cmd.args[0])
		if err != nil {
			return fmt.Errorf("limit must be a number")
		}
		limit = parsed
	}

	posts, err := s.db.GetPostsForUser(
		context.Background(),
		database.GetPostsForUserParams{
			UserID: user.ID,
			Limit:  int32(limit),
		},
	)
	if err != nil {
		return err
	}

	for _, post := range posts {
		fmt.Printf("\n%s\n", post.Title)
		fmt.Printf("%s\n", post.Url)
	}

	return nil
}

func middlewareLoggedIn(
	handler func(s *state, cmd command, user database.User) error,
) func(*state, command) error {

	return func(s *state, cmd command) error {
		ctx := context.Background()

		if s.cfg.CurrentUserName == "" {
			return fmt.Errorf("no user logged in")
		}

		user, err := s.db.GetUser(ctx, s.cfg.CurrentUserName)
		if err != nil {
			return err
		}

		return handler(s, cmd, user)
	}
}

func scrapeFeeds(s *state) {
	ctx := context.Background()

	feed, err := s.db.GetNextFeedToFetch(ctx)
	if err != nil {
		fmt.Println("error getting next feed:", err)
		return
	}

	fmt.Println("Fetching feed:", feed.Name)

	err = s.db.MarkFeedFetched(ctx, feed.ID)
	if err != nil {
		fmt.Println("error marking feed fetched:", err)
		return
	}

	feedData, err := rss.FetchFeed(ctx, feed.Url)
	if err != nil {
		fmt.Println("error fetching feed:", err)
		return
	}

	for _, item := range feedData.Channel.Item {

	publishedAt, err := parsePublishedAt(item.PubDate)
	if err != nil {
		fmt.Println("error parsing date:", err)
		continue
	}

	err = s.db.CreatePost(ctx, database.CreatePostParams{
		ID:          uuid.New(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Title:       item.Title,
		Url:         item.Link,
		Description: sql.NullString{String: item.Description, Valid: item.Description != ""},
		PublishedAt: sql.NullTime{Time: publishedAt, Valid: !publishedAt.IsZero()},
		FeedID:      feed.ID,
	})

	if err != nil {
		// Ignore duplicate URL
		if strings.Contains(err.Error(), "duplicate") {
			continue
		}
		fmt.Println("error saving post:", err)
	}
}
}

func parsePublishedAt(dateStr string) (time.Time, error) {
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, dateStr)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
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
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerUsers)
	cmds.register("agg", handlerAgg)
	cmds.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

	cmd := command{
		name: os.Args[1],
		args: os.Args[2:],
	}

	if err := cmds.run(s, cmd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
