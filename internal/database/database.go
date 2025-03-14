package database

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
	"laile/internal/log"
	dbmodels "laile/internal/postgresql"
)

type Service interface {
	Health() map[string]string
	Queries() *dbmodels.Queries
	BeginTx(ctx context.Context) (Transaction, error)
	GetConn(ctx context.Context) (Connection, error)
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

type dBConnectionConfig struct {
	Database string
	Password string
	Username string
	Port     string
	Host     string
}

func (c dBConnectionConfig) DSN() string {
	hostWithPort := net.JoinHostPort(c.Host, c.Port)
	return fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", c.Username, c.Password, hostWithPort, c.Database)
}

func New() Service {
	ctx := context.Background()
	connectionConfig := dBConnectionConfig{
		Database: os.Getenv("DB_DATABASE"),
		Password: os.Getenv("DB_PASSWORD"),
		Username: os.Getenv("DB_USERNAME"),
		Port:     os.Getenv("DB_PORT"),
		Host:     os.Getenv("DB_HOST"),
	}

	connStr := connectionConfig.DSN()

	log.Logger.DebugContext(ctx, "connecting to database", "connection_string", connStr)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Logger.Error("cannot parse db config", slog.Any("error", err))
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Logger.Error("cannot create db pool", slog.Any("error", err))
	}

	s := &service{pool: pool}
	return s
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
