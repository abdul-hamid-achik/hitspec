package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
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

// LoadAndExportDotEnv parses a .env file, returns key-value pairs,
// and exports them to the OS environment for ${VAR} resolution.
// Variables are only exported if not already set in the OS environment.
func LoadAndExportDotEnv(path string) (map[string]string, error) {
	vars, err := LoadDotEnv(path)
	if err != nil {
		return nil, err
	}

	// Export to OS environment (only if not already set)
	for k, v := range vars {
		if os.Getenv(k) == "" {
			_ = os.Setenv(k, v) // Error ignored: only fails for invalid key names
		}
	}

	return vars, nil
}
