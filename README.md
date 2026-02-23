# Gator

Gator is a CLI RSS feed aggregator written in Go.

It fetches RSS feeds in the background, stores posts in a PostgreSQL database, and allows you to browse them from the terminal.

---

## Requirements

Before running Gator, you must have installed:

- Go (1.21+ recommended)
- PostgreSQL

You will also need:

- goose (for database migrations)
- sqlc (for generating database code)

---

## Installation

Since Go programs are statically compiled binaries, you can install the CLI globally using:

go install github.com/arthurnagem/gator@latest

After installing, you can run:

gator

from anywhere in your terminal.

> Note: `go run .` is only for development.  
> `gator` is for production use after installation.

---

## Database Setup

1. Create a PostgreSQL database:

createdb gator

2. Run database migrations:

goose up

---

## Configuration

Create a config file at:

~/.gatorconfig.json

Example:

{
  "db_url": "postgres://username:password@localhost:5432/gator?sslmode=disable",
  "current_user_name": ""
}

---

## Usage

### Register a user

gator register <username>

### Login

gator login <username>

### Add a feed

gator addfeed "<name>" "<url>"

Example:

gator addfeed "TechCrunch" "https://techcrunch.com/feed/"

### Follow a feed

gator follow <url>

### Start the aggregator

gator agg 1m

This runs continuously and fetches feeds every 1 minute.
Stop with Ctrl+C.

### Browse posts

gator browse

Default limit is 2 posts.

gator browse 10

Fetch the 10 most recent posts.