// Package db provides database connectivity for SQL assertions in hitspec.
// It supports SQLite, PostgreSQL, and MySQL databases.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	// SQLite driver
	_ "github.com/mattn/go-sqlite3"
)

// QueryResult represents the result of a database query
type QueryResult struct {
	Columns []string
	Rows    []map[string]interface{}
}

// Client represents a database client
type Client struct {
	db          *sql.DB
	driverName  string
	dataSource  string
	queryTimout time.Duration
}

// NewClient creates a new database client from a connection string
func NewClient(connectionString string) (*Client, error) {
	driver, dsn, err := parseConnectionString(connectionString)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &Client{
		db:          db,
		driverName:  driver,
		dataSource:  dsn,
		queryTimout: 30 * time.Second,
	}, nil
}

// Close closes the database connection
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// Query executes a SQL query and returns the result
func (c *Client) Query(query string) (*QueryResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.queryTimout)
	defer cancel()

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	result := &QueryResult{
		Columns: columns,
		Rows:    make([]map[string]interface{}, 0),
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for better handling
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return result, nil
}

// parseConnectionString parses a connection string into driver and DSN
// Supported formats:
// - sqlite://path/to/db.sqlite
// - sqlite:./test.db
// - postgres://user:pass@host:port/dbname
// - mysql://user:pass@host:port/dbname
func parseConnectionString(connStr string) (driver string, dsn string, err error) {
	connStr = strings.TrimSpace(connStr)

	// Handle sqlite:// and sqlite: prefixes
	if strings.HasPrefix(connStr, "sqlite://") {
		return "sqlite3", strings.TrimPrefix(connStr, "sqlite://"), nil
	}
	if strings.HasPrefix(connStr, "sqlite:") {
		return "sqlite3", strings.TrimPrefix(connStr, "sqlite:"), nil
	}

	// Parse as URL for postgres/mysql
	u, err := url.Parse(connStr)
	if err != nil {
		return "", "", fmt.Errorf("invalid connection string: %w", err)
	}

	switch u.Scheme {
	case "postgres", "postgresql":
		// PostgreSQL DSN format
		return "postgres", connStr, nil
	case "mysql":
		// MySQL DSN format: user:pass@tcp(host:port)/dbname
		host := u.Host
		if u.Port() == "" {
			host = host + ":3306"
		}
		password, _ := u.User.Password()
		dsn = fmt.Sprintf("%s:%s@tcp(%s)%s", u.User.Username(), password, host, u.Path)
		if u.RawQuery != "" {
			dsn += "?" + u.RawQuery
		}
		return "mysql", dsn, nil
	default:
		return "", "", fmt.Errorf("unsupported database scheme: %s", u.Scheme)
	}
}
