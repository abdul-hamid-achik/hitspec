package clientmgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// SampleConfigFile is the filename of the generated configuration.
const SampleConfigFile = "hitspec.yaml"

// SampleRequestFile is the filename of the generated example request file.
const SampleRequestFile = "example.http"

// SampleConfigYAML is the default project configuration written by both
// `hitspec init` and the in-app "generate sample project" action, so the two
// stay in lockstep.
const SampleConfigYAML = `defaultEnvironment: dev
timeout: 30s
retries: 0
followRedirects: true
maxRedirects: 10
validateSSL: true
headers:
  User-Agent: hitspec/1.0
environments:
  dev:
    baseUrl: http://localhost:3000
  staging:
    baseUrl: https://staging.api.example.com
  prod:
    baseUrl: https://api.example.com
`

// SampleRequestHTTP is the default example request file. It exercises the most
// common hitspec features: variables, assertions, captures, and dependencies.
const SampleRequestHTTP = `@baseUrl = {{baseUrl}}

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

// ScaffoldSample writes a starter hitspec project (config + example request
// file) into the workspace. Existing files are left untouched. It returns the
// list of relative paths that were created.
func (m *Manager) ScaffoldSample(ctx context.Context) ([]string, error) {
	_ = ctx
	if err := m.requireWritable(); err != nil {
		return nil, err
	}

	type sampleFile struct {
		name    string
		content string
	}
	want := []sampleFile{
		{SampleConfigFile, SampleConfigYAML},
		{SampleRequestFile, SampleRequestHTTP},
	}

	created := make([]string, 0, len(want))
	for _, f := range want {
		absPath, err := m.absPath(f.name)
		if err != nil {
			return created, err
		}
		if _, err := os.Stat(absPath); err == nil {
			continue // already present — never clobber
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return created, err
		}
		m.suppressWatch(absPath)
		if err := os.WriteFile(absPath, []byte(f.content), 0o644); err != nil {
			return created, err
		}
		rel := m.relPath(absPath)
		m.publish("file_changed", FileEvent{Path: rel, Operation: "created", Timestamp: nowISO()})
		created = append(created, rel)
	}

	if len(created) == 0 {
		return nil, fmt.Errorf("sample files already exist")
	}
	return created, nil
}
