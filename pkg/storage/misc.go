package storage

import (
	"cmp"
	"maps"

	"github.com/sirrobot01/decypharr/internal/config"
)

// HandleExistingEntryMerge merges an incoming entry with an existing one that
// shares the same infohash. This preserves placements, files, and tags from
// the existing entry that the incoming entry may not know about.
func HandleExistingEntryMerge(existing, incoming *Entry) *Entry {
	if incoming.Protocol == config.ProtocolNZB {
		return incoming
	}
	incoming.Providers = mergeProviders(existing.Providers, incoming.Providers)
	incoming.Files = pruneUnservedFiles(mergeFiles(existing.Files, incoming.Files), incoming.Providers)
	incoming.ActiveProvider = cmp.Or(incoming.ActiveProvider, existing.ActiveProvider)
	incoming.Tags = mergeTags(existing.Tags, incoming.Tags)

	return incoming
}

func mergeProviders(existing, incoming map[string]*ProviderEntry) map[string]*ProviderEntry {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	merged := maps.Clone(existing)
	for k, v := range incoming {
		if current, exists := merged[k]; !exists || v.AddedAt.After(current.AddedAt) {
			merged[k] = v
		}
	}

	return merged
}

// pruneUnservedFiles drops files no placement has anymore. They are left over
// from an older add, e.g. when Real-Debrid repacks a torrent into one archive.
func pruneUnservedFiles(files map[string]*File, providers map[string]*ProviderEntry) map[string]*File {
	served := make(map[string]bool)
	for _, p := range providers {
		for name := range p.Files {
			served[name] = true
		}
	}
	if len(served) == 0 {
		return files // placements without file lists, nothing to compare against
	}
	maps.DeleteFunc(files, func(name string, _ *File) bool { return !served[name] })
	return files
}

func mergeFiles(existing, incoming map[string]*File) map[string]*File {
	if existing == nil {
		return incoming
	}
	if incoming == nil {
		return existing
	}

	merged := maps.Clone(existing)
	for k, v := range incoming {
		if current, exists := merged[k]; !exists || v.AddedOn.After(current.AddedOn) {
			merged[k] = v
		}
	}

	return merged
}

func mergeTags(existing, incoming []string) []string {
	if len(existing) == 0 {
		return incoming
	}
	if len(incoming) == 0 {
		return existing
	}

	tagSet := make(map[string]bool)
	for _, tag := range existing {
		tagSet[tag] = true
	}
	for _, tag := range incoming {
		tagSet[tag] = true
	}

	merged := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		merged = append(merged, tag)
	}
	return merged
}
