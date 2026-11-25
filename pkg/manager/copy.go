package manager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

const maxCopyTransfers = 50

// CopyTransfer is a library item being downloaded to a folder on this device.
type CopyTransfer struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Files      int       `json:"files"`
	Size       int64     `json:"size"`
	Downloaded int64     `json:"downloaded"`
	Speed      int64     `json:"speed"`
	Status     string    `json:"status"` // downloading, completed, error
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
}

type copyTransfers struct {
	mu   sync.Mutex
	list []*CopyTransfer // newest last
}

func (c *copyTransfers) add(t *CopyTransfer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append(c.list, t)
	// Drop the oldest finished transfers once the list is full.
	for len(c.list) > maxCopyTransfers {
		i := slices.IndexFunc(c.list, func(t *CopyTransfer) bool { return t.Status != "downloading" })
		if i < 0 {
			break
		}
		c.list = slices.Delete(c.list, i, i+1)
	}
}

func (c *copyTransfers) update(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn()
}

func (c *copyTransfers) snapshot() []CopyTransfer {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]CopyTransfer, 0, len(c.list))
	for i := len(c.list) - 1; i >= 0; i-- {
		out = append(out, *c.list[i])
	}
	return out
}

// CopyTransfers lists device downloads started from the browser, newest first.
func (m *Manager) CopyTransfers() []CopyTransfer {
	return m.copies.snapshot()
}

// CopyToFolder downloads files of a library item into folder/<item name> in the
// background. No file names means the whole item.
func (m *Manager) CopyToFolder(itemName string, fileNames []string, folder string) (*CopyTransfer, error) {
	if !filepath.IsAbs(folder) {
		return nil, fmt.Errorf("folder must be an absolute path")
	}
	item, err := m.storage.GetEntryItem(itemName)
	if err != nil {
		return nil, fmt.Errorf("item %s not found", itemName)
	}
	if len(fileNames) == 0 {
		for name, f := range item.Files {
			if !f.Deleted {
				fileNames = append(fileNames, name)
			}
		}
	}

	target := filepath.Join(filepath.Clean(folder), itemName)
	files := make([]*storage.File, 0, len(fileNames))
	var size int64
	for _, name := range fileNames {
		f, err := item.GetFile(name)
		if err != nil {
			return nil, fmt.Errorf("file %s not found in %s", name, itemName)
		}
		// Names come from the debrid, keep them from escaping the target folder.
		if rel, err := filepath.Rel(folder, filepath.Join(target, f.Name)); err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("invalid file name %q", f.Name)
		}
		files = append(files, f)
		size += f.Size
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%s has no files", itemName)
	}
	if err := os.MkdirAll(target, os.ModePerm); err != nil {
		return nil, fmt.Errorf("create %s: %w", target, err)
	}

	t := &CopyTransfer{
		ID: uuid.NewString(), Name: itemName, Path: target, Files: len(files),
		Size: size, Status: "downloading", StartedAt: time.Now(),
	}
	m.copies.add(t)
	go m.runCopy(t, files)
	return t, nil
}

func (m *Manager) runCopy(t *CopyTransfer, files []*storage.File) {
	d := m.downloader
	ctx := d.operationContext()
	var err error
	for _, f := range files {
		if err = m.copyFile(t, f); err != nil || ctx.Err() != nil {
			break
		}
	}
	m.copies.update(func() {
		t.Speed = 0
		if err != nil {
			t.Status, t.Error = "error", err.Error()
		} else {
			t.Status = "completed"
		}
	})
	if err != nil {
		d.logger.Error().Err(err).Str("item", t.Name).Str("path", t.Path).Msg("Device download failed")
	} else {
		d.logger.Info().Str("item", t.Name).Str("path", t.Path).Msg("Device download finished")
	}
}

func (m *Manager) copyFile(t *CopyTransfer, f *storage.File) error {
	d := m.downloader
	ctx := d.operationContext()
	dest := filepath.Join(t.Path, f.Name)

	entry, err := m.GetEntry(f.InfoHash)
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}

	if entry.IsNZB() {
		if m.usenet == nil {
			return fmt.Errorf("usenet is not configured")
		}
		out, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer out.Close()
		var last int64
		err = m.usenet.Download(ctx, f.InfoHash, f.Name, out, func(downloaded, speed int64) {
			m.copies.update(func() { t.Downloaded += downloaded - last; t.Speed = speed })
			last = downloaded
		})
		if err != nil {
			_ = os.Remove(dest)
			return fmt.Errorf("%s: %w", f.Name, err)
		}
		return nil
	}

	link, err := d.resolveLinkWithRetry(ctx, entry, f.Name)
	if err != nil {
		return fmt.Errorf("%s: %w", f.Name, err)
	}
	return d.localDownloader(ctx, link.DownloadLink, dest, f.ByteRange, func(downloaded, speed int64) {
		m.copies.update(func() { t.Downloaded += downloaded; t.Speed = speed })
	})
}
