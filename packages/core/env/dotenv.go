package env

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv parses a .env file and returns key-value pairs.
// Supports: KEY=value, KEY="quoted value", KEY='single quoted', # comments
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
