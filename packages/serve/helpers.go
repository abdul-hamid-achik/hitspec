package serve

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// maxRequestBody is the maximum size for JSON request bodies (10MB).
const maxRequestBody = 10 * 1024 * 1024

func readJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBody)
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

const idChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = idChars[rand.Intn(len(idChars))]
	}
	return string(b)
}

// isPathWithin checks that resolved stays within base directory.
func isPathWithin(base, target string) bool {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) || absTarget == absBase
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
