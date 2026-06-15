package tui

import (
	"context"
	"testing"
)

// TestLoadCommandClosures executes the read-only command builders end-to-end
// against a real (temp-dir) manager so their closures — not just their wiring —
// are exercised. The scaffold runs first so there's a file to load.
func TestLoadCommandClosures(t *testing.T) {
	ctx := context.Background()
	mgr := newTestManager(t)

	// scaffoldSampleCmd writes a starter project; assert it succeeds.
	if msg, ok := scaffoldSampleCmd(ctx, mgr)().(simpleMsg); !ok {
		t.Fatalf("scaffold -> %T, want simpleMsg", scaffoldSampleCmd(ctx, mgr)())
	} else if msg.err != nil {
		t.Fatalf("scaffold errored: %v", msg.err)
	}

	files, ok := loadFilesCmd(ctx, mgr)().(filesLoadedMsg)
	if !ok || files.err != nil {
		t.Fatalf("loadFilesCmd -> %#v", files)
	}
	if len(files.files) == 0 {
		t.Fatal("loadFilesCmd returned no files after scaffold")
	}

	first := files.files[0].RelativePath
	loaded, ok := loadFileCmd(ctx, mgr, first)().(fileLoadedMsg)
	if !ok || loaded.err != nil {
		t.Fatalf("loadFileCmd(%q) -> %#v", first, loaded)
	}
	if loaded.raw == "" || loaded.parsed == nil {
		t.Fatalf("loadFileCmd(%q) returned empty content/parse", first)
	}

	if cfg, ok := loadConfigCmd(ctx, mgr)().(configMsg); !ok || cfg.err != nil {
		t.Fatalf("loadConfigCmd -> %#v", cfg)
	}
	if h, ok := loadHistoryCmd(ctx, mgr)().(historyMsg); !ok || h.err != nil {
		t.Fatalf("loadHistoryCmd -> %#v", h)
	}
	if c, ok := loadCookiesCmd(ctx, mgr)().(cookiesMsg); !ok || c.err != nil {
		t.Fatalf("loadCookiesCmd -> %#v", c)
	}
}

// TestLoadFileCmdMissing covers the error arm: reading a file that isn't there
// surfaces the error on the message rather than panicking.
func TestLoadFileCmdMissing(t *testing.T) {
	ctx := context.Background()
	msg, ok := loadFileCmd(ctx, newTestManager(t), "does-not-exist.http")().(fileLoadedMsg)
	if !ok {
		t.Fatalf("loadFileCmd missing -> %T, want fileLoadedMsg", msg)
	}
	if msg.err == nil {
		t.Fatal("loadFileCmd on a missing file should surface an error")
	}
}
