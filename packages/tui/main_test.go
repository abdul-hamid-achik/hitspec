package tui

import (
	"os"
	"testing"

	"github.com/abdul-hamid-achik/hitspec/v2/internal/testhome"
)

// TestMain isolates HOME so tests never touch the real ~/.hitspec (run
// history, cookie jar).
func TestMain(m *testing.M) {
	os.Exit(testhome.Run(m))
}
