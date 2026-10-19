package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultipartFileNames(t *testing.T) {
	tests := []struct {
		name, line, field, path string
	}{
		{"named", "file avatar @./me.png", "avatar", "./me.png"},
		{"named with extra spaces", "file   avatar   @  ./me.png", "avatar", "./me.png"},
		{"default name with @", "file @./me.png", "file", "./me.png"},
		{"default name without @", "file ./me.png", "file", "./me.png"},
		{"path with spaces", "file ./my pics/me.png", "file", "./my pics/me.png"},
		{"named path with spaces", "file photo @./my pics/me.png", "photo", "./my pics/me.png"},
		{"name with brackets", "file files[0] @./a.txt", "files[0]", "./a.txt"},
		{"variable path", "file doc @{{dir}}/a.txt", "doc", "{{dir}}/a.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse("### up\nPOST http://x/\n\n>>>multipart\n"+tt.line+"\n<<<\n", "t.http")
			require.NoError(t, err)
			parts := f.Requests[0].Body.Multipart
			require.Len(t, parts, 1)
			assert.Equal(t, MultipartFieldFile, parts[0].Type)
			assert.Equal(t, tt.field, parts[0].Name)
			assert.Equal(t, tt.path, parts[0].Path)
		})
	}
}

func TestAuthAWSSessionToken(t *testing.T) {
	f, err := Parse("### a\n# @auth aws AK, SK, us-east-1, s3, tok/en==\nGET http://x/\n", "t.http")
	require.NoError(t, err)
	auth := f.Requests[0].Metadata.Auth
	assert.Equal(t, AuthAWS, auth.Type)
	assert.Equal(t, []string{"AK", "SK", "us-east-1", "s3", "tok/en=="}, auth.Params)

	f, err = Parse("### a\n# @auth aws {{k}} {{s}} {{r}} {{svc}} {{token}}\nGET http://x/\n", "t.http")
	require.NoError(t, err)
	assert.Len(t, f.Requests[0].Metadata.Auth.Params, 5)

	_, err = Parse("### a\n# @auth aws a, b, c, d, e, f\nGET http://x/\n", "t.http")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "@auth aws expects 4 arguments")
	assert.Contains(t, err.Error(), "sessionToken")
}
