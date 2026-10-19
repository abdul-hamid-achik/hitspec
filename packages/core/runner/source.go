package runner

import (
	"context"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

type sourceKey struct{}

// WithSource returns a context that makes RunFileContext parse src instead of
// reading the file from disk. The path passed to RunFileContext is still the
// file's real location, so environments, .env files, body-file references and
// snapshots resolve next to it, and results report that path. Interactive
// clients use it to run an editor buffer that has not been saved.
func WithSource(ctx context.Context, src string) context.Context {
	return context.WithValue(ctx, sourceKey{}, src)
}

// parseForRun parses the unsaved source carried by ctx (see WithSource), or the
// file on disk when there is none.
func parseForRun(ctx context.Context, path string) (*parser.File, error) {
	if src, ok := ctx.Value(sourceKey{}).(string); ok {
		return parser.Parse(src, path)
	}
	return parser.ParseFile(path)
}
