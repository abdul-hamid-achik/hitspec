package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new hitspec project",
	Long: `Initialize a new hitspec project in the current directory.

This creates:
  - hitspec.yaml   - Configuration file with environments
  - example.http   - Example test file

Examples:
  hitspec init
  hitspec init --force`,
	RunE: initCommand,
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing files")
}

func initCommand(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	configFile := filepath.Join(cwd, "hitspec.yaml")
	exampleFile := filepath.Join(cwd, "example.http")

	if !forceInit {
		for _, f := range []string{configFile, exampleFile} {
			if _, err := os.Stat(f); err == nil {
				return fmt.Errorf("file already exists: %s (use --force to overwrite)", f)
			}
		}
	}

	// Combined config with environments (single file, proper YAML format)
	configContent := map[string]any{
		"defaultEnvironment": "dev",
		"timeout":            "30s",
		"retries":            0,
		"followRedirects":    true,
		"maxRedirects":       10,
		"validateSSL":        true,
		"headers": map[string]string{
			"User-Agent": "hitspec/1.0",
		},
		"environments": map[string]map[string]string{
			"dev": {
				"baseUrl": "http://localhost:3000",
			},
			"staging": {
				"baseUrl": "https://staging.api.example.com",
			},
			"prod": {
				"baseUrl": "https://api.example.com",
			},
		},
	}

	configYAML, _ := yaml.Marshal(configContent)
	if err := os.WriteFile(configFile, configYAML, 0644); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", configFile)

	exampleContent := `@baseUrl = {{baseUrl}}

### Get health status
# @name healthCheck
# @description Check if the API is running
# @tags smoke

GET {{baseUrl}}/health

>>>
expect status 200
<<<

### Create a resource
# @name createResource
# @tags crud

POST {{baseUrl}}/resources
Content-Type: application/json

{
  "name": "Test Resource",
  "description": "Created by hitspec"
}

>>>
expect status 201
expect body.id exists
expect body.name == "Test Resource"
<<<

>>>capture
resourceId from body.id
<<<

### Get the created resource
# @name getResource
# @tags crud
# @depends createResource

GET {{baseUrl}}/resources/{{createResource.resourceId}}

>>>
expect status 200
expect body.id == {{createResource.resourceId}}
<<<
`

	if err := os.WriteFile(exampleFile, []byte(exampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create example file: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Created: %s\n", exampleFile)

	fmt.Fprintf(cmd.OutOrStdout(), "\nhitspec project initialized!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Run 'hitspec run example.http' to execute the example tests.\n")

	return nil
}
