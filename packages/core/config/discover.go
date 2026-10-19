package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Loaded is a configuration together with where it came from.
type Loaded struct {
	// Config is never nil; it is DefaultConfig() when no file was found.
	Config *Config
	// Path is the config file that was read, "" when none was found.
	Path string
	// Warnings are the non-fatal problems found in the file, such as unknown
	// keys. Each reads "<file>:<line>: <message>" and carries no "warning:"
	// prefix; the caller decides how to print them.
	Warnings []string
}

// Load reads the config file at path.
func Load(path string) (*Loaded, error) {
	return loadFile(path)
}

// Discover looks for hitspec.yaml / hitspec.yml in startDir and then in each
// parent directory, and loads the first one found. The search is bounded: it
// never reads a config from the user's home directory or above, and it stops
// at the first directory that contains .git (the project root). When nothing
// is found it returns the defaults and an empty Path.
func Discover(startDir string) (*Loaded, error) {
	return loaded(FindConfigPathUp(startDir))
}

// DiscoverWithin is Discover that additionally never leaves root: startDir's
// parents are searched only up to and including root. A startDir outside root
// finds nothing. Agent-facing callers (the MCP server) use it so a workspace
// never inherits configuration from a directory above it.
func DiscoverWithin(startDir, root string) (*Loaded, error) {
	return loaded(FindConfigPathWithin(startDir, root))
}

func loaded(path string) (*Loaded, error) {
	if path == "" {
		return &Loaded{Config: DefaultConfig()}, nil
	}
	return loadFile(path)
}

// FindConfigPathUp returns the config file Discover would load, or "".
func FindConfigPathUp(startDir string) string {
	return findUp(startDir, "")
}

// FindConfigPathWithin returns the config file DiscoverWithin would load, or
// "".
func FindConfigPathWithin(startDir, root string) string {
	if root == "" {
		return ""
	}
	return findUp(startDir, root)
}

// findUp walks from startDir toward the filesystem root and returns the first
// config file. It stops (without reading) at the user's home directory, stops
// after the first directory holding .git, and, when root is set, after root.
func findUp(startDir, root string) string {
	if startDir == "" {
		startDir = "."
	}
	dir := canonical(startDir)
	if dir == "" {
		return ""
	}
	var stop string
	if root != "" {
		if stop = canonical(root); stop == "" || !within(dir, stop) {
			return ""
		}
	}
	home := homeDir()
	for {
		if home != "" && dir == home {
			return ""
		}
		if path := FindConfigPath(dir); path != "" {
			return path
		}
		if dir == stop || exists(filepath.Join(dir, ".git")) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// DisplayPath spells an absolute config path for messages: relative to the
// working directory when that is shorter, absolute otherwise.
func DisplayPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil || !filepath.IsAbs(path) {
		return path
	}
	best := path
	for _, base := range []string{cwd, canonical(cwd)} {
		if base == "" {
			continue
		}
		if rel, err := filepath.Rel(base, path); err == nil && len(rel) < len(best) {
			best = rel
		}
	}
	return best
}

// canonical returns the absolute, symlink-free spelling of dir ("" on error).
func canonical(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return canonical(home)
}

// within reports whether dir is root or lies below it.
func within(dir, root string) bool {
	rel, err := filepath.Rel(root, dir)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
