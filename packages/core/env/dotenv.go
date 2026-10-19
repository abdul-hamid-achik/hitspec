package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// LoadDotEnv parses a .env file and returns key-value pairs.
// Supports: KEY=value, KEY="quoted value", KEY='single quoted', # comments
// Note: This does NOT export to OS environment. Use LoadAndExportDotEnv if you
// need ${VAR} syntax to work in config files loaded after the .env file.
func LoadDotEnv(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open env file: %w", err)
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first = sign
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue // Skip lines without =
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		// Skip if key is empty
		if key == "" {
			continue
		}

		// Handle quoted values
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		result[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading env file: %w", err)
	}

	return result, nil
}

// dotenvExports remembers, per env file, which process environment variables
// LoadAndExportDotEnv created and the value it put there, so a later load of
// the same file can refresh them without touching variables the user set.
var (
	dotenvMu      sync.Mutex
	dotenvExports = map[string]map[string]string{}
)

// LoadAndExportDotEnv parses a .env file, returns key-value pairs,
// and exports them to the OS environment for ${VAR} and {{$env(VAR)}}
// resolution.
//
// A variable that is already set in the OS environment is left alone (the
// shell wins over the file), with one exception: one that an earlier call for
// the same file exported and nobody has changed since. Loading the file again
// refreshes it to the file's current value and unsets it when the file no
// longer defines it. That is what lets a long-lived process (hitspec serve,
// the studio, `run --watch`) pick up an edited .env instead of keeping the
// first value it read.
func LoadAndExportDotEnv(path string) (map[string]string, error) {
	vars, err := LoadDotEnv(path)
	if err != nil {
		return nil, err
	}

	id := path
	if abs, err := filepath.Abs(path); err == nil {
		id = abs
	}

	dotenvMu.Lock()
	defer dotenvMu.Unlock()
	prev := dotenvExports[id]
	next := make(map[string]string, len(vars))
	for k, v := range vars {
		cur := os.Getenv(k)
		old, ours := prev[k]
		// Not set, or still the value this file exported earlier.
		if cur == "" || (ours && cur == old) {
			_ = os.Setenv(k, v) // Error ignored: only fails for invalid key names
			next[k] = v
		}
	}
	for k, old := range prev {
		if _, still := vars[k]; !still && os.Getenv(k) == old {
			_ = os.Unsetenv(k)
		}
	}
	dotenvExports[id] = next

	return vars, nil
}
