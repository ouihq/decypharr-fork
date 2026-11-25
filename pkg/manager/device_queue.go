package manager

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
)

// errDevicePaused cancels a device download that the user paused.
var errDevicePaused = errors.New("paused")

// DeviceQueueItem is one download to this device as shown in the UI.
type DeviceQueueItem struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Status   string `json:"status"`   // running, waiting, paused
	Position int    `json:"position"` // 1-based place in the queue
}

type deviceItem struct {
	key, name string
	paused    bool
	cancel    context.CancelCauseFunc // set while it holds a slot
}

// deviceQueue orders downloads to this device and lets at most
// max_active_downloads of them run. Paused items wait until resumed.
// shortcut: order and paused state live in memory, a restart starts every unfinished download again; persist them if that matters.
type deviceQueue struct {
	mu      sync.Mutex
	items   []*deviceItem
	changed chan struct{}
}

func (q *deviceQueue) notifyLocked() {
	if q.changed != nil {
		close(q.changed)
	}
	q.changed = make(chan struct{})
}

func (q *deviceQueue) canStartLocked(it *deviceItem) bool {
	if it.paused {
		return false
	}
	running := 0
	for _, o := range q.items {
		if o.cancel != nil {
			running++
		}
	}
	if running >= max(config.Get().MaxActiveDownloads, 1) {
		return false
	}
	// Only the first waiting item may take a free slot.
	for _, o := range q.items {
		if o == it {
			return true
		}
		if o.cancel == nil && !o.paused {
			return false
		}
	}
	return false
}

// run calls fn once the item gets a slot. A paused fn waits for resume and
// runs again, so fn must pick up where it left off.
func (q *deviceQueue) run(ctx context.Context, key, name string, fn func(context.Context) error) error {
	it := &deviceItem{key: key, name: name}
	q.mu.Lock()
	q.items = append(q.items, it)
	q.notifyLocked()
	q.mu.Unlock()
	defer func() {
		q.mu.Lock()
		q.items = slices.DeleteFunc(q.items, func(o *deviceItem) bool { return o == it })
		q.notifyLocked()
		q.mu.Unlock()
	}()

	for {
		runCtx, err := q.acquire(ctx, it)
		if err != nil {
			return err
		}
		err = fn(runCtx)
		q.mu.Lock()
		it.cancel(nil)
		it.cancel = nil
		paused := it.paused
		q.notifyLocked()
		q.mu.Unlock()
		if err != nil && paused && ctx.Err() == nil {
			continue
		}
		return err
	}
}

func (q *deviceQueue) acquire(ctx context.Context, it *deviceItem) (context.Context, error) {
	for {
		q.mu.Lock()
		if q.changed == nil {
			q.changed = make(chan struct{})
		}
		if q.canStartLocked(it) {
			runCtx, cancel := context.WithCancelCause(ctx)
			it.cancel = cancel
			q.notifyLocked()
			q.mu.Unlock()
			return runCtx, nil
		}
		changed := q.changed
		q.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-changed:
		case <-time.After(5 * time.Second): // picks up a changed max_active_downloads
		}
	}
}

func (q *deviceQueue) list() []DeviceQueueItem {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]DeviceQueueItem, 0, len(q.items))
	for i, it := range q.items {
		status := "waiting"
		switch {
		case it.paused:
			status = "paused"
		case it.cancel != nil:
			status = "running"
		}
		out = append(out, DeviceQueueItem{Key: it.key, Name: it.name, Status: status, Position: i + 1})
	}
	return out
}

func (q *deviceQueue) status(key string) string {
	for _, it := range q.list() {
		if it.Key == key {
			return it.Status
		}
	}
	return ""
}

// apply runs a user action on an item: pause, resume, up, down or top.
func (q *deviceQueue) apply(key, action string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.IndexFunc(q.items, func(o *deviceItem) bool { return o.key == key })
	if i < 0 {
		return false
	}
	it := q.items[i]
	switch action {
	case "pause":
		it.paused = true
		if it.cancel != nil {
			it.cancel(errDevicePaused)
		}
	case "resume":
		it.paused = false
	case "up":
		if i > 0 {
			q.items[i-1], q.items[i] = q.items[i], q.items[i-1]
		}
	case "down":
		if i < len(q.items)-1 {
			q.items[i+1], q.items[i] = q.items[i], q.items[i+1]
		}
	case "top":
		q.items = append([]*deviceItem{it}, slices.Delete(q.items, i, i+1)...)
	default:
		return false
	}
	q.notifyLocked()
	return true
}

// DeviceQueue lists downloads to this device in queue order.
func (m *Manager) DeviceQueue() []DeviceQueueItem {
	return m.downloader.queue.list()
}

// DeviceQueueAction pauses, resumes or moves a download to this device.
func (m *Manager) DeviceQueueAction(key, action string) bool {
	return m.downloader.queue.apply(key, action)
}
