package server

import (
	"fmt"
	"net/http"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"laile/internal/config"
	"laile/internal/database"
	db_models "laile/internal/postgresql"
)

const (
	ReadTimeout  = 10 * time.Second
	WriteTimeout = 30 * time.Second
)

type Server struct {
	port    int
	db      database.Service
	queries *db_models.Queries
	config  *config.Config
}

func NewServer(db database.Service, config *config.Config) *http.Server {
	port := config.Settings.ListenerPort
	newServer := &Server{
		port:    port,
		db:      db,
		queries: db.Queries(),
		config:  config,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", newServer.port),
		Handler:      newServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
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
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
	}

	return server
}
