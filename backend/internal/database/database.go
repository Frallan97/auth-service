package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	*pgxpool.Pool
}

func Connect(databaseURL string) (*DB, error) {
	ctx := context.Background()

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	db.Pool.Close()
}

// RunMigrations runs database migrations on startup
func RunMigrations(databaseURL string) error {
	// Get migrations path from environment or use default
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		// Default path in container (matches Dockerfile WORKDIR /root/)
		migrationsPath = "file:///root/migrations"
	}

	m, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer func() {
		_, closeErr := m.Close()
		if closeErr != nil {
			log.Printf("Warning: failed to close migrate instance: %v", closeErr)
		}
	}()

	// Run migrations
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// LoadActiveOrigins loads all active allowed origins from the database
func (db *DB) LoadActiveOrigins(ctx context.Context) ([]string, error) {
	query := `
		SELECT origin
		FROM allowed_origins
		WHERE is_active = true
		ORDER BY origin
	`

	rows, err := db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query allowed origins: %w", err)
	}
	defer rows.Close()

	var origins []string
	for rows.Next() {
		var origin string
		if err := rows.Scan(&origin); err != nil {
			return nil, fmt.Errorf("failed to scan origin: %w", err)
		}
		origins = append(origins, origin)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating origins: %w", err)
	}

	return origins, nil
}
