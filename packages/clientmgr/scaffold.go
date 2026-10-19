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
timeout: 30000
retries: 0
followRedirects: true
maxRedirects: 10
validateSSL: true
headers:
  User-Agent: hitspec
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
//
// The >>>mock blocks make the file servable as-is by `hitspec mock`, so a new
// project runs green without any API of its own:
//
//	hitspec mock example.http --port 3000    (terminal 1)
//	hitspec run example.http                 (terminal 2)
//
// `hitspec run` ignores the mock blocks, so pointing baseUrl at a real API in
// hitspec.yaml keeps working.
const SampleRequestHTTP = `# hitspec example. Serve it with the built-in mock server, then run it:
#   hitspec mock example.http --port 3000     (terminal 1)
#   hitspec run example.http                  (terminal 2)
# To test a real API instead, change baseUrl in hitspec.yaml.

@baseUrl = {{baseUrl}}

### Get health status
# @name healthCheck
# @description Check if the API is running
# @tags smoke

GET {{baseUrl}}/health

>>>
expect status 200
expect body.status == "ok"
<<<

>>>mock
{"status": "ok"}
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

>>>mock
{"id": 1, "name": "Test Resource", "description": "Created by hitspec"}
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

>>>mock
{"id": 1, "name": "Test Resource", "description": "Created by hitspec"}
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
		m.search.invalidate(absPath)
		rel := m.relPath(absPath)
		m.publish("file_changed", FileEvent{Path: rel, Operation: "created", Timestamp: nowISO()})
		created = append(created, rel)
	}

	if len(created) == 0 {
		return nil, fmt.Errorf("sample files already exist")
	}
	return created, nil
}
