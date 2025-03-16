package main

import (
	"context"
	"fmt"

	"github.com/joho/godotenv"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/event"
	"laile/internal/log"
	"laile/internal/server"
)

func main() {
	log.InitLogger()
	appConfig, err := config.LoadMainConfig()
	if err != nil {
		panic(fmt.Sprintf("cannot load config: %s", err))
	}
	_ = godotenv.Load()
	db, err := database.New(&appConfig.Database)
	if err != nil {
		panic(fmt.Sprintf("cannot create database: %s", err))
	}

	// TODO: add app server config settings
	go func() {
		err = event.ProcessEvents(context.Background(), db, appConfig)
		if err != nil {
			panic(fmt.Sprintf("cannot start event processor: %s", err))
		}
	}()

	// Start AdminServer in a goroutine
	go func() {
		adminServer := server.NewAdminServer(db, appConfig)
		adminServerErr := adminServer.ListenAndServe()
		if adminServerErr != nil {
			panic(fmt.Sprintf("cannot start admin server: %s", adminServerErr))
		}
	}()

	// Start ingress server (this blocks)
	ingressServer := server.NewServer(db, appConfig)
	err = ingressServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start ingress server: %s", err))
	}
}
