package env

import (
	"os"
	"regexp"
)

var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// resolveEnvVarsWith replaces ${VAR} patterns with the values lookup returns.
func resolveEnvVarsWith(value any, lookup func(string) string) any {
	str, ok := value.(string)
	if !ok {
		return value
	}
	return envVarPattern.ReplaceAllStringFunc(str, func(match string) string {
		varName := match[2 : len(match)-1] // Extract VAR from ${VAR}
		return lookup(varName)
	})
}

type Environment struct {
	Name      string
	Variables map[string]any
}

func LoadEnvironment(dir, envName string, configEnvs map[string]map[string]any) (*Environment, error) {
	return LoadEnvironmentWithLookup(dir, envName, configEnvs, os.Getenv)
}

// LoadEnvironmentWithLookup is LoadEnvironment with the ${VAR} expansion in the
// config's environments section routed through lookup instead of os.Getenv, so
// an isolated resolver (see WithIsolatedEnvironment) can keep config files from
// reading the process environment.
func LoadEnvironmentWithLookup(dir, envName string, configEnvs map[string]map[string]any, lookup func(string) string) (*Environment, error) {
	env := &Environment{
		Name:      envName,
		Variables: make(map[string]any),
	}

	// Load from hitspec.yaml environments section
	if configEnvs != nil {
		if vars, ok := configEnvs[envName]; ok {
			for k, v := range vars {
				env.Variables[k] = resolveEnvVarsWith(v, lookup)
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
