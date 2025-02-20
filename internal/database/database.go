package database

import (
	"context"
	"fmt"
	db_models "laile/internal/postgresql"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/joho/godotenv/autoload"
)

type Service interface {
	Health() map[string]string
	Queries() *db_models.Queries
	BeginTx(ctx context.Context) (Transaction, error)
	GetConn(ctx context.Context) (Connection, error)
}

type Transaction interface {
	Queries() *db_models.Queries
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	RawTx() pgx.Tx
}

type Connection interface {
	Queries() *db_models.Queries
	Release()
}

type service struct {
	pool *pgxpool.Pool
}

type transaction struct {
	tx pgx.Tx
	q  *db_models.Queries
}

type connection struct {
	conn *pgxpool.Conn
	q    *db_models.Queries
}

var (
	database = os.Getenv("DB_DATABASE")
	password = os.Getenv("DB_PASSWORD")
	username = os.Getenv("DB_USERNAME")
	port     = os.Getenv("DB_PORT")
	host     = os.Getenv("DB_HOST")
)

func New() Service {
	ctx := context.Background()

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", username, password, host, port, database)
	log.Println(connStr)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal(err)
	}

	s := &service{pool: pool}
	return s
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.pool.Ping(ctx)
	if err != nil {
		log.Println(err)
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

func (s *service) Queries() *db_models.Queries {
	return db_models.New(s.pool)
}

func (s *service) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	return &transaction{
		tx: tx,
		q:  db_models.New(tx),
	}, nil
}

func (t *transaction) Queries() *db_models.Queries {
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
		q:    db_models.New(conn),
	}, nil
}

func (c *connection) Queries() *db_models.Queries {
	return c.q
}

func (c *connection) Release() {
	c.conn.Release()
}
