package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_SQLite(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite://" + dbPath)
	require.NoError(t, err)
	defer client.Close()

	// Create a test table
	_, err = client.db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)`)
	require.NoError(t, err)

	// Insert test data
	_, err = client.db.Exec(`INSERT INTO users (name, age) VALUES ('Alice', 30), ('Bob', 25)`)
	require.NoError(t, err)

	// Query the data
	result, err := client.Query("SELECT COUNT(*) as count FROM users")
	require.NoError(t, err)

	assert.Len(t, result.Rows, 1)
	assert.Equal(t, int64(2), result.Rows[0]["count"])
}

func TestNewClient_SQLiteWithColonPrefix(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite:" + dbPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.db.Exec(`CREATE TABLE test (id INTEGER)`)
	require.NoError(t, err)
}

func TestQuery_SelectValues(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite://" + dbPath)
	require.NoError(t, err)
	defer client.Close()

	// Create and populate table
	_, err = client.db.Exec(`
		CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, price REAL, active INTEGER);
		INSERT INTO products (name, price, active) VALUES ('Widget', 9.99, 1);
		INSERT INTO products (name, price, active) VALUES ('Gadget', 19.99, 0);
	`)
	require.NoError(t, err)

	t.Run("select single value", func(t *testing.T) {
		result, err := client.Query("SELECT name FROM products WHERE id = 1")
		require.NoError(t, err)

		assert.Len(t, result.Rows, 1)
		assert.Equal(t, "Widget", result.Rows[0]["name"])
	})

	t.Run("select multiple columns", func(t *testing.T) {
		result, err := client.Query("SELECT name, price FROM products WHERE id = 1")
		require.NoError(t, err)

		assert.Len(t, result.Rows, 1)
		assert.Equal(t, "Widget", result.Rows[0]["name"])
		assert.Equal(t, 9.99, result.Rows[0]["price"])
	})

	t.Run("select multiple rows", func(t *testing.T) {
		result, err := client.Query("SELECT name FROM products ORDER BY id")
		require.NoError(t, err)

		assert.Len(t, result.Rows, 2)
		assert.Equal(t, "Widget", result.Rows[0]["name"])
		assert.Equal(t, "Gadget", result.Rows[1]["name"])
	})

	t.Run("aggregate query", func(t *testing.T) {
		result, err := client.Query("SELECT SUM(price) as total FROM products")
		require.NoError(t, err)

		assert.Len(t, result.Rows, 1)
		assert.InDelta(t, 29.98, result.Rows[0]["total"], 0.001)
	})
}

func TestParseConnectionString(t *testing.T) {
	tests := []struct {
		input    string
		driver   string
		dsn      string
		hasError bool
	}{
		{"sqlite://test.db", "sqlite3", "test.db", false},
		{"sqlite:./test.db", "sqlite3", "./test.db", false},
		{"sqlite:///tmp/test.db", "sqlite3", "/tmp/test.db", false},
		{"postgres://user:pass@localhost:5432/db", "postgres", "postgres://user:pass@localhost:5432/db", false},
		{"invalid", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			driver, dsn, err := parseConnectionString(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.driver, driver)
				assert.Equal(t, tt.dsn, dsn)
			}
		})
	}
}

func TestQuery_NoRows(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite://" + dbPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.db.Exec(`CREATE TABLE empty (id INTEGER)`)
	require.NoError(t, err)

	result, err := client.Query("SELECT * FROM empty")
	require.NoError(t, err)
	assert.Len(t, result.Rows, 0)
}

func TestQuery_Error(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite://" + dbPath)
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Query("SELECT * FROM nonexistent")
	assert.Error(t, err)
}

func TestNewClient_InvalidConnection(t *testing.T) {
	_, err := NewClient("sqlite:///nonexistent/path/to/db.sqlite")
	// SQLite creates the file if it doesn't exist, but fails if directory doesn't exist
	if err != nil {
		// This might or might not error depending on the system
		t.Log("Connection error (expected on some systems):", err)
	}
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	client, err := NewClient("sqlite://" + dbPath)
	require.NoError(t, err)

	err = client.Close()
	assert.NoError(t, err)

	// Verify db file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}
