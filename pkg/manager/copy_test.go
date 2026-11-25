package manager

import "testing"

func TestCopyTransfersKeepsActiveWhenFull(t *testing.T) {
	var c copyTransfers
	c.add(&CopyTransfer{ID: "active", Status: "downloading"})
	for range maxCopyTransfers {
		c.add(&CopyTransfer{Status: "completed"})
	}
	list := c.snapshot()
	if len(list) != maxCopyTransfers {
		t.Fatalf("len = %d, want %d", len(list), maxCopyTransfers)
	}
	if list[len(list)-1].ID != "active" {
		t.Fatal("active transfer was dropped")
	}
}
