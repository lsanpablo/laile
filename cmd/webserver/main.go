package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/monitoring"
	"laile/internal/server"
	"log/slog"
)

func main() {
	// Set up logging and load config
	slog.SetDefault(monitoring.Logger)
	conf, err := config.LoadMainConfig()
	if err != nil {
		panic(fmt.Sprintf("cannot load config: %s", err))
	}
	_ = godotenv.Load()
	db := database.New()

	// Start AdminServer in a goroutine
	go func() {
		adminServer := server.NewAdminServer(db, conf)
		err := adminServer.ListenAndServe()
		if err != nil {
			panic(fmt.Sprintf("cannot start admin server: %s", err))
		}
	}()

	// Start ingress server (this blocks)
	ingressServer := server.NewServer(db, conf)
	err = ingressServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start ingress server: %s", err))
	}
}
