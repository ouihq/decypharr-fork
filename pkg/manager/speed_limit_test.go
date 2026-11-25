package manager

import (
	"context"
	"testing"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"golang.org/x/time/rate"
)

func setSpeedLimit(t *testing.T, limit string) {
	t.Helper()
	if _, err := config.Update(func(c *config.Config) error { c.DownloadSpeedLimit = limit; return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestSpeedLimiterAppliesNewLimitDuringWait(t *testing.T) {
	config.Reset()
	config.SetConfigPath(t.TempDir())
	t.Cleanup(config.Reset)

	lim := speedLimiter{rate.NewLimiter(rate.Inf, localDownloadBufferSize)}
	setSpeedLimit(t, "100KB")
	// Use up the burst so the next wait is paced by the limit.
	if err := lim.WaitN(context.Background(), localDownloadBufferSize); err != nil {
		t.Fatal(err)
	}

	// A full buffer at 100KB/s would take about 10s. Removing the limit
	// shortly after must let it finish right away.
	go func() {
		time.Sleep(200 * time.Millisecond)
		setSpeedLimit(t, "")
	}()
	start := time.Now()
	if err := lim.WaitN(context.Background(), localDownloadBufferSize); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("wait took %s after the limit was removed", took)
	}

	// Waits larger than the burst at a high limit must not fail.
	setSpeedLimit(t, "500MB")
	if err := lim.WaitN(context.Background(), 4*localDownloadBufferSize); err != nil {
		t.Fatal(err)
	}
}
