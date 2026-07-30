package transfer

import (
	"context"
	"sync"
	"testing"
	"time"
)

type fakeFileClient struct {
	mu      sync.Mutex
	started []string
	startCh chan string
	release chan struct{}
}

func (f *fakeFileClient) UploadFile(ctx context.Context, _, key, _ string, progress func(done, total int64)) error {
	f.mu.Lock()
	f.started = append(f.started, key)
	f.mu.Unlock()
	if f.startCh != nil {
		f.startCh <- key
	}
	if progress != nil {
		progress(5, 10)
	}
	if f.release != nil && key == "hold" {
		select {
		case <-f.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if progress != nil {
		progress(10, 10)
	}
	return nil
}

func (f *fakeFileClient) DownloadFile(ctx context.Context, bucket, key, destination string, progress func(done, total int64)) error {
	return f.UploadFile(ctx, bucket, key, destination, progress)
}

func (f *fakeFileClient) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.started...)
}

func waitForState(t *testing.T, manager *Manager, id string, want State) Item {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, item := range manager.Snapshot() {
			if item.ID == id && item.State == want {
				return item
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("transfer %s did not reach state %v", id, want)
	return Item{}
}

func TestCancelWaitingTransferNeverRuns(t *testing.T) {
	client := &fakeFileClient{startCh: make(chan string, 2), release: make(chan struct{})}
	manager := New(1)
	group := manager.NewGroup("test")
	first := manager.AddUpload(client, "a", "b", "hold", "hold", group, "test")
	if key := <-client.startCh; key != "hold" {
		t.Fatalf("first started %q", key)
	}
	waitForState(t, manager, first, Running)

	second := manager.AddUpload(client, "a", "b", "never", "never", group, "test")
	manager.Cancel(second)
	waitForState(t, manager, second, Cancelled)
	manager.CloseGroup(group)
	close(client.release)
	waitForState(t, manager, first, Done)

	calls := client.calls()
	if len(calls) != 1 || calls[0] != "hold" {
		t.Fatalf("cancelled transfer ran: %#v", calls)
	}
}

func TestProgressAndGroupLifecycle(t *testing.T) {
	client := &fakeFileClient{}
	manager := New(1)
	group := manager.NewGroup("one")
	if manager.GroupClosed(group) {
		t.Fatal("new group should remain open while producers enqueue")
	}
	id := manager.AddUpload(client, "a", "b", "file", "file", group, "one")
	manager.CloseGroup(group)
	if !manager.GroupClosed(group) {
		t.Fatal("closed group was not reported closed")
	}
	item := waitForState(t, manager, id, Done)
	if item.Total != 10 || item.Completed != 10 || item.Progress() != 1 {
		t.Fatalf("unexpected progress: %#v", item)
	}
}

func TestCancelRunningTransfer(t *testing.T) {
	client := &fakeFileClient{startCh: make(chan string, 1), release: make(chan struct{})}
	manager := New(1)
	group := manager.NewGroup("cancel")
	id := manager.AddUpload(client, "a", "b", "hold", "hold", group, "cancel")
	<-client.startCh
	waitForState(t, manager, id, Running)
	manager.Cancel(id)
	manager.CloseGroup(group)
	item := waitForState(t, manager, id, Cancelled)
	if item.Err == nil {
		t.Fatal("cancelled transfer should retain a cancellation error")
	}
}
