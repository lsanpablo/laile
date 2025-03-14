package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/event"
	"laile/internal/log"
)

func main() {
	log.InitLogger()
	conf, err := config.LoadMainConfig()
	if err != nil {
		panic(fmt.Sprintf("cannot load config: %s", err))
	}
	_ = godotenv.Load()
	db := database.New()

	// Run the event processor. Assuming this call blocks while processing events.
	event.ProcessEvents(context.Background(), db, conf)

	// If ProcessEvents were to return immediately, add a blocking select:
	// select {}
}
