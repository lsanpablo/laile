package server

import (
	"fmt"
	db_models "laile/internal/postgresql"
	"net/http"
	"time"

	"laile/internal/config"
	"laile/internal/database"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port    int
	db      database.Service
	queries *db_models.Queries
	config  *config.Config
}

func NewServer(db database.Service, config *config.Config) *http.Server {
	port := config.Settings.ListenerPort
	NewServer := &Server{
		port:    port,
		db:      db,
		queries: db.Queries(),
		config:  config,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}

func NewAdminServer(db database.Service, config *config.Config) *http.Server {
	port := config.Settings.AdminPort
	adminServer := &Server{
		port:    port,
		db:      db,
		queries: db.Queries(),
		config:  config,
	}

	// Declare Admin Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", adminServer.port),
		Handler:      adminServer.RegisterAdminRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
