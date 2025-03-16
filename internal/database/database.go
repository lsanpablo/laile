package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"laile/internal/config"
	"laile/internal/log"
	dbmodels "laile/internal/postgresql"
)

type Service interface {
	Health() map[string]string
	Queries() *dbmodels.Queries
	BeginTx(ctx context.Context) (Transaction, error)
	GetConn(ctx context.Context) (Connection, error)
	Close()
}

func Rollback(ctx context.Context, tx Transaction) {
	err := tx.Rollback(ctx)
	if err != nil {
		log.Logger.ErrorContext(ctx, "failed to rollback transaction", slog.Any("error", err))
	}
}

type Transaction interface {
	Queries() *dbmodels.Queries
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	RawTx() pgx.Tx
}

type Connection interface {
	Queries() *dbmodels.Queries
	Release()
	RawConn() *pgxpool.Conn
}

type service struct {
	pool *pgxpool.Pool
}

type transaction struct {
	tx pgx.Tx
	q  *dbmodels.Queries
}

type connection struct {
	conn *pgxpool.Conn
	q    *dbmodels.Queries
}

// New creates a new database service using the provided configuration.
func New(dbConfig *config.DatabaseConfig) (Service, error) {
	ctx := context.Background()

	// Validate required connection parameters
	if dbConfig.Database == "" || dbConfig.Host == "" {
		return nil, errors.New("missing required database connection parameters")
	}

	// Get the DSN string from the poolConfig
	connStr := dbConfig.GetDSN()

	log.Logger.DebugContext(ctx, "connecting to database", "connection_string", connStr)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Logger.ErrorContext(ctx, "cannot parse db poolConfig", slog.Any("error", err))
		return nil, fmt.Errorf("failed to parse database configuration: %w", err)
	}

	// Set pool configuration from the provided poolConfig
	poolConfig.MaxConns = dbConfig.MaxConns
	poolConfig.MinConns = dbConfig.MinConns

	// Parse durations from string configurations
	maxConnLifetime, err := time.ParseDuration(dbConfig.MaxConnLifetime)
	if err != nil {
		log.Logger.WarnContext(ctx, "Invalid max_conn_lifetime, using default",
			"value", dbConfig.MaxConnLifetime,
			"default", poolConfig.MaxConnLifetime)
	} else {
		poolConfig.MaxConnLifetime = maxConnLifetime
	}

	maxConnIdleTime, err := time.ParseDuration(dbConfig.MaxConnIdleTime)
	if err != nil {
		log.Logger.WarnContext(ctx, "Invalid max_conn_idle_time, using default",
			"value", dbConfig.MaxConnIdleTime,
			"default", poolConfig.MaxConnIdleTime)
	} else {
		poolConfig.MaxConnIdleTime = maxConnIdleTime
	}

	healthCheckPeriod, err := time.ParseDuration(dbConfig.HealthCheckPeriod)
	if err != nil {
		log.Logger.WarnContext(ctx, "Invalid health_check_period, using default",
			"value", dbConfig.HealthCheckPeriod,
			"default", poolConfig.HealthCheckPeriod)
	} else {
		poolConfig.HealthCheckPeriod = healthCheckPeriod
	}

	// Log the final connection pool configuration
	log.Logger.InfoContext(ctx, "Database connection pool configuration",
		"max_conns", poolConfig.MaxConns,
		"min_conns", poolConfig.MinConns,
		"max_conn_lifetime", poolConfig.MaxConnLifetime,
		"max_conn_idle_time", poolConfig.MaxConnIdleTime,
		"health_check_period", poolConfig.HealthCheckPeriod,
		"ssl_mode", dbConfig.SSLMode)

	// Create connection pool with timeout
	connCtx, cancel := context.WithTimeout(ctx, dbConfig.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connCtx, poolConfig)
	if err != nil {
		log.Logger.ErrorContext(ctx, "cannot create db pool", slog.Any("error", err))
		return nil, fmt.Errorf("failed to create database connection pool: %w", err)
	}

	// Verify connection is working
	err = pool.Ping(connCtx)
	if err != nil {
		pool.Close()
		log.Logger.ErrorContext(ctx, "failed to ping database", slog.Any("error", err))
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Logger.InfoContext(ctx, "Successfully connected to database")
	return &service{pool: pool}, nil
}

func (s *service) Health() map[string]string {
	const defaultHealthcheckTimeout = 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), defaultHealthcheckTimeout)
	defer cancel()

	err := s.pool.Ping(ctx)
	if err != nil {
		log.Logger.Error("Failed to ping database", slog.Any("error", err))
		return map[string]string{
			"message": "It's not healthy",
		}
	}

	return map[string]string{
		"message": "It's healthy",
	}
}

func (s *service) Close() {
	s.pool.Close()
}

func (s *service) Queries() *dbmodels.Queries {
	return dbmodels.New(s.pool)
}

func (s *service) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	return &transaction{
		tx: tx,
		q:  dbmodels.New(tx),
	}, nil
}

func (t *transaction) Queries() *dbmodels.Queries {
	return t.q
}

func (t *transaction) Commit(ctx context.Context) error {
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}
	return nil
}

func (t *transaction) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil {
		return fmt.Errorf("rolling back transaction: %w", err)
	}
	return nil
}

func (t *transaction) RawTx() pgx.Tx {
	return t.tx
}

func (s *service) GetConn(ctx context.Context) (Connection, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquiring connection: %w", err)
	}

	return &connection{
		conn: conn,
		q:    dbmodels.New(conn),
	}, nil
}

func (c *connection) Queries() *dbmodels.Queries {
	return c.q
}

func (c *connection) Release() {
	c.conn.Release()
}

func (c *connection) RawConn() *pgxpool.Conn {
	return c.conn
}
