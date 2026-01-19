package env

import (
	"os"
)

type Environment struct {
	Name      string
	Variables map[string]any
}

func LoadEnvironment(dir, envName string, configEnvs map[string]map[string]any) (*Environment, error) {
	env := &Environment{
		Name:      envName,
		Variables: make(map[string]any),
	}

	// Load from hitspec.yaml environments section
	if configEnvs != nil {
		if vars, ok := configEnvs[envName]; ok {
			for k, v := range vars {
				env.Variables[k] = v
			}
		}
	}

	return env, nil
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
