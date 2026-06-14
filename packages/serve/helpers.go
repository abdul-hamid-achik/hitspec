package serve

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // headers already sent; can't write error response
}

// maxRequestBody is the maximum size for JSON request bodies (10MB).
const maxRequestBody = 10 * 1024 * 1024

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	// nil ResponseWriter is acceptable — MaxBytesReader returns an error
	// to the reader when the limit is exceeded, which Decode propagates.
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBody)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

const idChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateID() string {
	b := make([]byte, 8)
	max := big.NewInt(int64(len(idChars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// Fallback: use index 0 rather than panic
			b[i] = idChars[0]
			continue
		}
		b[i] = idChars[n.Int64()]
	}
	return string(b)
}

// isPathWithin checks that resolved stays within base directory.
// It resolves symlinks to prevent directory escape via symlink traversal.
func isPathWithin(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}

	if resolved, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = resolved
	}
	if resolved, err := filepath.EvalSymlinks(absTarget); err == nil {
		absTarget = resolved
	} else if rel, relErr := filepath.Rel(base, target); relErr == nil {
		absTarget = filepath.Join(absBase, rel)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// collectHitspecFiles walks dir and returns all .http/.hitspec files.
func collectHitspecFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && isHitspecFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func isHitspecFile(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".http" || ext == ".hitspec"
}

// apiError is a standard JSON error response.
type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: http.StatusText(status), Message: msg})
}
