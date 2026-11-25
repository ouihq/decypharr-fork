package config

import (
	"errors"
	"testing"
)

func TestFileFilterFor(t *testing.T) {
	c := &Config{AllowedExt: []string{"mkv"}, MinFileSize: "1KB"}

	arr := c.FileFilterFor(Arr{AllowedExt: []string{"flac"}, MaxFileSize: "1MB"})
	if err := arr.Allow("a.flac", 2048); err != nil {
		t.Fatalf("arr type: %v", err)
	}
	if err := arr.Allow("a.mkv", 2048); !errors.Is(err, ErrFileExtNotAllowed) {
		t.Fatalf("global type leaked into arr: %v", err)
	}
	if arr.Allow("a.flac", 10) == nil || arr.Allow("a.flac", 2<<20) == nil {
		t.Fatal("size limits not applied")
	}

	global := c.FileFilterFor(Arr{})
	if global.Allow("a.mkv", 2048) != nil || global.Allow("a.iso", 2048) == nil {
		t.Fatal("empty arr should use global rules")
	}

	manual := c.ManualFileFilter()
	if manual.Allow("a.iso", 2048) != nil || manual.Allow("a.iso", 10) == nil {
		t.Fatal("manual filter should skip types but keep sizes")
	}
}
