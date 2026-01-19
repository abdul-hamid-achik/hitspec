package env

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Environment struct {
	Name      string
	Variables map[string]any
}

type EnvFile struct {
	Environments map[string]map[string]any
}

func LoadEnvironment(dir, envName string, configEnvs map[string]map[string]any) (*Environment, error) {
	env := &Environment{
		Name:      envName,
		Variables: make(map[string]any),
	}

	// First load from hitspec.yaml environments section (lowest precedence)
	if configEnvs != nil {
		if vars, ok := configEnvs[envName]; ok {
			for k, v := range vars {
				env.Variables[k] = v
			}
		}
	}

	// Then overlay from .hitspec.env.json
	mainFile := filepath.Join(dir, ".hitspec.env.json")
	if err := loadEnvFile(mainFile, envName, env.Variables); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Finally overlay from .hitspec.env.local.json (highest precedence)
	localFile := filepath.Join(dir, ".hitspec.env.local.json")
	if err := loadEnvFile(localFile, envName, env.Variables); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return env, nil
}

func loadEnvFile(path, envName string, target map[string]any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var envFile map[string]map[string]any
	if err := json.Unmarshal(data, &envFile); err != nil {
		return err
	}

	if vars, ok := envFile[envName]; ok {
		for k, v := range vars {
			target[k] = v
		}
	}

	return nil
}

func LoadEnvironmentFromFile(path, envName string) (*Environment, error) {
	return LoadEnvironment(filepath.Dir(path), envName, nil)
}

func MergeVariables(sources ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, src := range sources {
		for k, v := range src {
			result[k] = v
		}
	}
	return result
}

func LoadSystemEnv(prefix string) map[string]any {
	result := make(map[string]any)
	for _, e := range os.Environ() {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				key := e[:i]
				value := e[i+1:]
				if prefix == "" {
					result[key] = value
				} else if len(key) > len(prefix) && key[:len(prefix)] == prefix {
					result[key[len(prefix):]] = value
				}
				break
			}
		}
	}
	return result
}
