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
		panic(fmt.Sprintf("cannot load appConfig: %s", err))
	}
	_ = godotenv.Load()
	db := database.New()
	go event.ProcessEvents(context.Background(), db, appConfig)
	go func() {
		adminServer := server.NewAdminServer(db, appConfig)
		err = adminServer.ListenAndServe()
		if err != nil {
			panic(fmt.Sprintf("cannot start admin server: %s", err))
		}
	}()

	ingressServer := server.NewServer(db, appConfig)
	err = ingressServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start server: %s", err))
	}
}
