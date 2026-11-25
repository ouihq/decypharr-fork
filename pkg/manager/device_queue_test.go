package manager

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
)

func TestDeviceQueueOrderPauseResume(t *testing.T) {
	config.Reset()
	config.SetConfigPath(t.TempDir())
	t.Cleanup(config.Reset)
	if _, err := config.Update(func(c *config.Config) error { c.MaxActiveDownloads = 1; return nil }); err != nil {
		t.Fatal(err)
	}

	var q deviceQueue
	started := make(chan string, 10)
	release := map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{}), "c": make(chan struct{})}
	var aRuns atomic.Int32
	runItem := func(key string) {
		_ = q.run(context.Background(), key, key, func(ctx context.Context) error {
			if key == "a" && aRuns.Add(1) > 1 {
				started <- "a-resumed"
				return nil
			}
			started <- key
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-release[key]:
				return nil
			}
		})
	}
	next := func(want string) {
		t.Helper()
		select {
		case got := <-started:
			if got != want {
				t.Fatalf("started %q, want %q", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for %q", want)
		}
	}

	go runItem("a")
	next("a")
	go runItem("b")
	go runItem("c")
	for len(q.list()) < 3 {
		time.Sleep(10 * time.Millisecond)
	}
	q.apply("c", "top")   // c jumps ahead of b
	q.apply("a", "pause") // frees the only slot
	next("c")
	close(release["c"])
	next("b")
	close(release["b"])
	q.apply("a", "resume")
	next("a-resumed")
}
