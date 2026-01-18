// Package assertions provides test assertion functionality for hitspec.
//
// Supported assertions:
//   - Status code checks (expect status 200)
//   - Header validation (expect header Content-Type contains application/json)
//   - Body content checks (expect body contains "success")
//   - JSONPath queries (expect body.data.id exists)
//   - JSON Schema validation (expect schema ./schema.json)
//   - Length checks (expect body length 10)
//   - Type checks (expect body.items type array)
//
// Assertions support various operators: equals, contains, exists, matches, etc.
package assertions
