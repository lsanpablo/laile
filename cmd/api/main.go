package main

import (
	"context"
	"fmt"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/event"
	"laile/internal/monitoring"
	"laile/internal/server"
	"log/slog"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(monitoring.Logger)
	config, err := config.LoadMainConfig()
	if err != nil {
		panic(fmt.Sprintf("cannot load config: %s", err))
	}
	_ = godotenv.Load()
	db := database.New()
	eventsChan := make(chan string)
	go event.EventProcessor(context.Background(), eventsChan, db, config)
	go func() {
		adminServer := server.NewAdminServer(db, config)
		err = adminServer.ListenAndServe()
		if err != nil {
			panic(fmt.Sprintf("cannot start admin server: %s", err))
		}
	}()

	ingressServer := server.NewServer(db, config)
	err = ingressServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}

}
