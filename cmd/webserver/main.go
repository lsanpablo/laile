package main

import (
	"fmt"

	"github.com/joho/godotenv"
	"laile/internal/config"
	"laile/internal/database"
	"laile/internal/log"
	"laile/internal/server"
)

func main() {
	log.InitLogger()
	conf, err := config.LoadMainConfig()
	if err != nil {
		panic(fmt.Sprintf("cannot load config: %s", err))
	}
	_ = godotenv.Load()
	db := database.New()

	// Start AdminServer in a goroutine
	go func() {
		adminServer := server.NewAdminServer(db, conf)
		adminServerErr := adminServer.ListenAndServe()
		if adminServerErr != nil {
			panic(fmt.Sprintf("cannot start admin server: %s", adminServerErr))
		}
	}()

	// Start ingress server (this blocks)
	ingressServer := server.NewServer(db, conf)
	err = ingressServer.ListenAndServe()
	if err != nil {
		panic(fmt.Sprintf("cannot start ingress server: %s", err))
	}
}
