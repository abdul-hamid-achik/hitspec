package pathutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateWithinBase(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
	}{
		{
			name:    "valid path within base",
			path:    "/home/user/project/schema.json",
			baseDir: "/home/user/project",
			wantErr: false,
		},
		{
			name:    "path traversal",
			path:    "/home/user/project/../../../etc/passwd",
			baseDir: "/home/user/project",
			wantErr: true,
		},
		{
			name:    "empty base allows all",
			path:    "/any/path",
			baseDir: "",
			wantErr: false,
		},
		{
			name:    "exact match",
			path:    "/home/user/project",
			baseDir: "/home/user/project",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWithinBase(tt.path, tt.baseDir)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
