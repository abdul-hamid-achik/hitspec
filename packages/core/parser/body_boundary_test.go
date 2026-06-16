package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A request body followed directly by a >>>db block (no intervening assertion
// block) must not swallow the db block into the body. Regression test for the
// parseBody stop-condition gap.
func TestParseBody_StopsAtDBBlock(t *testing.T) {
	input := `POST http://x/users
Content-Type: application/json

{"name": "John"}

>>>db
query SELECT COUNT(*) FROM users
expect count > 0
<<<
`
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.NotNil(t, req.Body)
	require.Contains(t, req.Body.Raw, "John")
	require.NotContains(t, req.Body.Raw, "SELECT") // db block must not be in the body
	require.Len(t, req.DBAssertions, 1)
}

// Same boundary guarantee for >>>shell blocks following a body.
func TestParseBody_StopsAtShellBlock(t *testing.T) {
	input := `POST http://x/users
Content-Type: application/json

{"name": "John"}

>>>shell
echo hello
<<<
`
	file, err := Parse(input, "test.http")
	require.NoError(t, err)
	req := file.Requests[0]
	require.NotNil(t, req.Body)
	require.Contains(t, req.Body.Raw, "John")
	require.NotContains(t, req.Body.Raw, "echo") // shell block must not be in the body
	require.Len(t, req.ShellCommands, 1)
}
