package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var (
	ErrFileIsSample      = errors.New("file is sample")
	ErrFileExtNotAllowed = errors.New("file extension not allowed")
)

var (
	sampleMatch = `(?i)(^|[\s/\\])(sample|trailer|thumb|special|extras?)s?[-/]|(\((sample|trailer|thumb|special|extras?)s?\))|(-\s*(sample|trailer|thumb|special|extras?)s?)`
	sampleRegex = regexp.MustCompile(sampleMatch)
)

func isSample(path string) bool {
	filename := filepath.Base(path)
	if strings.HasSuffix(strings.ToLower(filename), "sample.mkv") {
		return true
	}
	return sampleRegex.MatchString(filename)
}

// FileFilter decides which files of a download are kept.
type FileFilter struct {
	AllowedExt   []string
	AnyExt       bool // skip the extension check
	MinSize      int64
	MaxSize      int64
	AllowSamples bool
}

// Allow returns why a file is skipped, or nil. A nil filter uses the global settings.
func (f *FileFilter) Allow(filename string, filesize int64) error {
	if f == nil {
		f = Get().FileFilter()
	}
	if !f.AllowSamples && isSample(filename) {
		return ErrFileIsSample
	}
	if !f.AnyExt && !slices.Contains(f.AllowedExt, strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")) {
		return ErrFileExtNotAllowed
	}
	// 0 size means the debrid hasn't reported it yet
	if filesize > 0 && (f.MinSize > 0 && filesize < f.MinSize || f.MaxSize > 0 && filesize > f.MaxSize) {
		return fmt.Errorf("file size %d is not allowed. expected range: [%d - %d]", filesize, f.MinSize, f.MaxSize)
	}
	return nil
}

// FileFilter returns the global file rules.
func (c *Config) FileFilter() *FileFilter {
	return &FileFilter{
		AllowedExt:   c.AllowedExt,
		MinSize:      c.GetMinFileSize(),
		MaxSize:      c.GetMaxFileSize(),
		AllowSamples: c.AllowSamples,
	}
}

// FileFilterFor returns the Arr's file rules. Each unset field uses the global value.
func (c *Config) FileFilterFor(a Arr) *FileFilter {
	f := c.FileFilter()
	if len(a.AllowedExt) > 0 {
		f.AllowedExt = a.AllowedExt
	}
	if s, err := ParseSize(a.MinFileSize); err == nil {
		f.MinSize = s
	}
	if s, err := ParseSize(a.MaxFileSize); err == nil {
		f.MaxSize = s
	}
	return f
}

// ManualFileFilter is used for downloads with no Arr behind them: any file type.
func (c *Config) ManualFileFilter() *FileFilter {
	f := c.FileFilter()
	f.AnyExt = true
	return f
}

func (c *Config) ValidateFileAllowed(filename string, filesize int64) error {
	return c.FileFilter().Allow(filename, filesize)
}

func getDefaultExtensions() []string {
	videoExts := strings.Split("webm,m4v,3gp,nsv,ty,strm,rm,rmvb,m3u,ifo,mov,qt,divx,xvid,bivx,nrg,pva,wmv,asf,asx,ogm,ogv,m2v,avi,bin,dat,dvr-ms,mpg,mpeg,mp4,avc,vp3,svq3,nuv,viv,dv,fli,flv,wpl,vob,mkv,mk3d,ts,wtv,m2ts", ",")
	musicExts := strings.Split("MP3,WAV,FLAC,OGG,WMA,AIFF,ALAC,M4A,APE,AC3,DTS,M4P,MID,MIDI,MKA,MP2,MPA,RA,VOC,WV,AMR", ",")

	// Combine both slices
	allExts := append(videoExts, musicExts...)

	// Convert to lowercase
	for i, ext := range allExts {
		allExts[i] = strings.ToLower(ext)
	}

	// Remove duplicates
	seen := make(map[string]struct{})
	var unique []string

	for _, ext := range allExts {
		if _, ok := seen[ext]; !ok {
			seen[ext] = struct{}{}
			unique = append(unique, ext)
		}
	}

	sort.Strings(unique)
	return unique
}

func ParseSize(sizeStr string) (int64, error) {
	// Accept a decimal comma too, e.g. "2,5MB".
	sizeStr = strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(sizeStr)), ",", ".")

	// Absolute size-based cache. Order matters: two-letter units must be
	// checked before the bare "B" suffix. ParseFloat below means decimal
	// values (e.g. "2.2TB", "1.5GB") are supported for every unit.
	multiplier := 1.0
	switch {
	case strings.HasSuffix(sizeStr, "PB"):
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "PB")
	case strings.HasSuffix(sizeStr, "TB"):
		multiplier = 1024 * 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "TB")
	case strings.HasSuffix(sizeStr, "GB"):
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	case strings.HasSuffix(sizeStr, "MB"):
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	case strings.HasSuffix(sizeStr, "KB"):
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	case strings.HasSuffix(sizeStr, "B"):
		sizeStr = strings.TrimSuffix(sizeStr, "B")
	}

	size, err := strconv.ParseFloat(strings.TrimSpace(sizeStr), 64)
	if err != nil {
		return 0, err
	}

	return int64(size * multiplier), nil
}
