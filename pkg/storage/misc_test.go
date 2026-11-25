package storage

import (
	"testing"
	"time"
)

func TestHandleExistingEntryMergeDropsUnservedFiles(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	existing := &Entry{
		Files: map[string]*File{"a.bin": {Name: "a.bin"}, "b.bin": {Name: "b.bin"}},
		Providers: map[string]*ProviderEntry{"realdebrid": {
			AddedAt: old, Files: map[string]*ProviderFile{"a.bin": {}, "b.bin": {}},
		}},
	}
	incoming := &Entry{
		Files: map[string]*File{"release.rar": {Name: "release.rar"}},
		Providers: map[string]*ProviderEntry{"realdebrid": {
			AddedAt: time.Now(), Files: map[string]*ProviderFile{"release.rar": {}},
		}},
	}

	merged := HandleExistingEntryMerge(existing, incoming)
	if len(merged.Files) != 1 || merged.Files["release.rar"] == nil {
		t.Fatalf("files = %v, want only release.rar", merged.Files)
	}
}
