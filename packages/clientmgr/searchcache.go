package clientmgr

import (
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/abdul-hamid-achik/hitspec/v2/packages/core/parser"
)

// parseFile is the parser entry point SearchRequests uses; tests swap it to
// count parses.
var parseFile = parser.ParseFile

// searchEntry is what search keeps of one parsed request.
type searchEntry struct {
	name, method, url string
	tags              []string
	line              int
	hay               string // lower-cased name, method, URL and tags
}

// searchFile is one cached parse, valid while the file keeps this mtime and size.
type searchFile struct {
	modTime time.Time
	size    int64
	entries []searchEntry // nil when the file did not parse
}

// searchCache remembers parsed request summaries per absolute path so a search
// per keystroke does not re-parse the whole workspace.
type searchCache struct {
	mu    sync.Mutex
	files map[string]searchFile
	// gen counts invalidations. A parse that started before one is not stored,
	// so a concurrent write cannot leave a stale entry behind.
	gen uint64
}

// lookup returns the cached entries for path when both its mtime and size still
// match, along with the generation to hand to store.
func (c *searchCache) lookup(path string, info os.FileInfo) ([]searchEntry, bool, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f, ok := c.files[path]
	if ok && f.modTime.Equal(info.ModTime()) && f.size == info.Size() {
		return f.entries, true, c.gen
	}
	return nil, false, c.gen
}

func (c *searchCache) store(path string, info os.FileInfo, entries []searchEntry, gen uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if gen != c.gen {
		return
	}
	if c.files == nil {
		c.files = make(map[string]searchFile)
	}
	c.files[path] = searchFile{modTime: info.ModTime(), size: info.Size(), entries: entries}
}

// invalidate drops the entries for paths. A quick edit can keep the same size
// and mtime, so every write path calls it explicitly.
func (c *searchCache) invalidate(paths ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	for _, p := range paths {
		delete(c.files, p)
	}
}

// retain drops entries for files that are no longer in the workspace.
func (c *searchCache) retain(present []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.files) == 0 {
		return
	}
	keep := make(map[string]struct{}, len(present))
	for _, p := range present {
		keep[p] = struct{}{}
	}
	for p := range c.files {
		if _, ok := keep[p]; !ok {
			delete(c.files, p)
		}
	}
}

// searchEntries returns the request summaries of path, parsing it only when
// the cache has no entry matching its current mtime and size. ok is false for
// a file that cannot be read or parsed.
func (c *searchCache) searchEntries(path string) (entries []searchEntry, ok bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	cached, hit, gen := c.lookup(path, info)
	if hit {
		return cached, cached != nil
	}
	parsed, perr := parseFile(path)
	if perr != nil {
		c.store(path, info, nil, gen)
		return nil, false
	}
	entries = make([]searchEntry, 0, len(parsed.Requests))
	for _, r := range parsed.Requests {
		entries = append(entries, searchEntry{
			name:   r.Name,
			method: r.Method,
			url:    r.URL,
			tags:   slices.Clone(r.Tags),
			line:   r.Line,
			hay:    strings.ToLower(strings.Join([]string{r.Name, r.Method, r.URL, strings.Join(r.Tags, " ")}, " ")),
		})
	}
	c.store(path, info, entries, gen)
	return entries, true
}
