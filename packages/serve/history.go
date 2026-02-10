package serve

import "sync"

const maxHistoryEntries = 100

// History is a thread-safe circular buffer of execution history entries.
type History struct {
	mu      sync.RWMutex
	entries []HistoryEntryDTO
}

// NewHistory creates a new History.
func NewHistory() *History {
	return &History{entries: make([]HistoryEntryDTO, 0, maxHistoryEntries)}
}

// Add appends an entry, evicting the oldest if at capacity.
func (h *History) Add(entry HistoryEntryDTO) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.entries) >= maxHistoryEntries {
		h.entries = h.entries[1:]
	}
	h.entries = append(h.entries, entry)
}

// Entries returns a copy of all entries (newest last).
func (h *History) Entries() []HistoryEntryDTO {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]HistoryEntryDTO, len(h.entries))
	copy(out, h.entries)
	return out
}

// Clear removes all history entries.
func (h *History) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries = h.entries[:0]
}
