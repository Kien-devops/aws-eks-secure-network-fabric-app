package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DBClient struct {
	Pool *pgxpool.Pool
}

type OrderRecord struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func ConnectPostgres(ctx context.Context, host, port, user, password, dbName, sslMode string) (*DBClient, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password='%s' dbname=%s sslmode=%s",
		host, port, user, password, dbName, sslMode)

	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping failed: %w", err)
	}

	log.Printf("Connected successfully to Aurora PostgreSQL at %s:%s/%s", host, port, dbName)
	return &DBClient{Pool: pool}, nil
}

func (d *DBClient) RunMigrations(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS orders (
		id VARCHAR(64) PRIMARY KEY,
		product_id VARCHAR(64) NOT NULL,
		quantity INT NOT NULL,
		amount NUMERIC(12, 2) NOT NULL,
		status VARCHAR(32) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
	`
	_, err := d.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("migration execution failed: %w", err)
	}
	log.Println("Aurora PostgreSQL table schema 'orders' verified/created.")
	return nil
}

func (d *DBClient) CreateOrder(ctx context.Context, order OrderRecord) error {
	query := `
	INSERT INTO orders (id, product_id, quantity, amount, status, created_at)
	VALUES ($1, $2, $3, $4, $5, $6);
	`
	_, err := d.Pool.Exec(ctx, query, order.ID, order.ProductID, order.Quantity, order.Amount, order.Status, order.CreatedAt)
	return err
}

func (d *DBClient) GetOrder(ctx context.Context, id string) (*OrderRecord, error) {
	query := `SELECT id, product_id, quantity, amount, status, created_at FROM orders WHERE id = $1;`
	var order OrderRecord
	err := d.Pool.QueryRow(ctx, query, id).Scan(
		&order.ID, &order.ProductID, &order.Quantity, &order.Amount, &order.Status, &order.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (d *DBClient) ListOrders(ctx context.Context, limit int) ([]OrderRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	query := `SELECT id, product_id, quantity, amount, status, created_at FROM orders ORDER BY created_at DESC LIMIT $1;`
	rows, err := d.Pool.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []OrderRecord
	for rows.Next() {
		var o OrderRecord
		if err := rows.Scan(&o.ID, &o.ProductID, &o.Quantity, &o.Amount, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}


