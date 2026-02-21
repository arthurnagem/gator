package main

import (
	"fmt"
	"log"

	"github.com/arthurnagem/gator/internal/config"
)

func main() {
	// 1. Read config
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Set user
	err = cfg.SetUser("arthurnagem")
	if err != nil {
		log.Fatal(err)
	}

	// 3. Read again
	updatedCfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Config contents:\n%+v\n", updatedCfg)
}
