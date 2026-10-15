package transfer

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Kind int

const (
	Upload Kind = iota
	Download
)

type State int

const (
	Waiting State = iota
	Running
	Done
	Failed
	Cancelled
)

type FileClient interface {
	UploadFile(ctx context.Context, bucket, key, path string, progress func(done, total int64)) error
	DownloadFile(ctx context.Context, bucket, key, destination string, progress func(done, total int64)) error
}

type Item struct {
	ID         string
	GroupID    string
	GroupLabel string
	Kind       Kind
	Account    string
	Bucket     string
	Key        string
	LocalPath  string
	Total      int64
	Completed  int64
	State      State
	Err        error
	StartedAt  time.Time
	FinishedAt time.Time

	client FileClient
	ctx    context.Context
	cancel context.CancelFunc
}

type Manager struct {
	mu          sync.RWMutex
	queue       chan *Item
	items       map[string]*Item
	order       []string
	groups      map[string]bool // false while producers are still enqueueing, true when closed
	workerCount int
	counter     atomic.Uint64
}

func New(workerCount int) *Manager {
	if workerCount < 1 {
		workerCount = 3
	}
	manager := &Manager{
		queue:       make(chan *Item, 4096),
		items:       map[string]*Item{},
		groups:      map[string]bool{},
		workerCount: workerCount,
	}
	for i := 0; i < workerCount; i++ {
		go manager.worker()
	}
	return manager
}

func (m *Manager) NewGroup(label string) string {
	id := fmt.Sprintf("group-%d", m.counter.Add(1)) + "-" + label
	m.mu.Lock()
	m.groups[id] = false
	m.mu.Unlock()
	return id
}

// CloseGroup marks a logical batch as fully enqueued. Completion notifications
// wait for this signal, so a fast first file cannot make a still-growing folder
// upload look finished.
func (m *Manager) CloseGroup(id string) {
	if id == "" {
		return
	}
	m.mu.Lock()
	if _, ok := m.groups[id]; ok {
		m.groups[id] = true
	}
	m.mu.Unlock()
}

func (m *Manager) GroupClosed(id string) bool {
	if id == "" {
		return true
	}
	m.mu.RLock()
	closed, ok := m.groups[id]
	m.mu.RUnlock()
	return !ok || closed
}

func (m *Manager) AddUpload(client FileClient, account, bucket, key, path, groupID, groupLabel string) string {
	item := m.newItem(Upload, client, account, bucket, key, path, groupID, groupLabel)
	m.enqueue(item)
	return item.ID
}

func (m *Manager) AddDownload(client FileClient, account, bucket, key, path, groupID, groupLabel string) string {
	item := m.newItem(Download, client, account, bucket, key, path, groupID, groupLabel)
	m.enqueue(item)
	return item.ID
}

func (m *Manager) newItem(kind Kind, client FileClient, account, bucket, key, path, groupID, groupLabel string) *Item {
	id := fmt.Sprintf("transfer-%d", m.counter.Add(1))
	ctx, cancel := context.WithCancel(context.Background())
	return &Item{
		ID: id, GroupID: groupID, GroupLabel: groupLabel, Kind: kind,
		Account: account, Bucket: bucket, Key: key, LocalPath: path,
		State: Waiting, client: client, ctx: ctx, cancel: cancel,
	}
}

func (m *Manager) enqueue(item *Item) {
	m.mu.Lock()
	m.items[item.ID] = item
	m.order = append(m.order, item.ID)
	m.mu.Unlock()
	m.queue <- item
}

func (m *Manager) worker() {
	for item := range m.queue {
		m.mu.Lock()
		current, ok := m.items[item.ID]
		if !ok || current.State != Waiting {
			m.mu.Unlock()
			continue
		}
		if current.ctx.Err() != nil {
			m.mu.Unlock()
			m.finish(item, Cancelled, current.ctx.Err())
			continue
		}
		current.State = Running
		current.StartedAt = time.Now()
		m.mu.Unlock()

		var err error
		switch item.Kind {
		case Upload:
			err = item.client.UploadFile(item.ctx, item.Bucket, item.Key, item.LocalPath, func(done, total int64) {
				m.progress(item.ID, done, total)
			})
		case Download:
			err = item.client.DownloadFile(item.ctx, item.Bucket, item.Key, item.LocalPath, func(done, total int64) {
				m.progress(item.ID, done, total)
			})
		}
		if err != nil {
			if item.ctx.Err() != nil {
				m.finish(item, Cancelled, item.ctx.Err())
			} else {
				m.finish(item, Failed, err)
			}
			continue
		}
		m.finish(item, Done, nil)
	}
}

func (m *Manager) progress(id string, done, total int64) {
	m.update(id, func(item *Item) {
		item.Completed = done
		if total > 0 {
			item.Total = total
		}
	})
}

func (m *Manager) finish(item *Item, state State, err error) {
	m.mu.Lock()
	current, ok := m.items[item.ID]
	if !ok || current.State == Done || current.State == Failed || current.State == Cancelled {
		m.mu.Unlock()
		return
	}
	current.State = state
	current.Err = err
	current.FinishedAt = time.Now()
	if state == Done && current.Total > 0 {
		current.Completed = current.Total
	}
	m.mu.Unlock()
}

func (m *Manager) update(id string, fn func(*Item)) {
	m.mu.Lock()
	if item, ok := m.items[id]; ok {
		fn(item)
	}
	m.mu.Unlock()
}

func (m *Manager) Snapshot() []Item {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Item, 0, len(m.order))
	for _, id := range m.order {
		if item, ok := m.items[id]; ok {
			out = append(out, cloneItem(item))
		}
	}
	return out
}

func cloneItem(item *Item) Item {
	if item == nil {
		return Item{}
	}
	copy := *item
	copy.client = nil
	copy.cancel = nil
	copy.ctx = nil
	return copy
}

func (m *Manager) Cancel(id string) {
	m.mu.RLock()
	item := m.items[id]
	if item == nil {
		m.mu.RUnlock()
		return
	}
	state := item.State
	cancel := item.cancel
	m.mu.RUnlock()
	if cancel == nil || (state != Waiting && state != Running) {
		return
	}
	cancel()
	if state == Waiting {
		m.finish(item, Cancelled, context.Canceled)
	}
}

func (m *Manager) ClearFinished() {
	m.mu.Lock()
	newOrder := m.order[:0]
	for _, id := range m.order {
		item := m.items[id]
		if item == nil {
			continue
		}
		if item.State == Done || item.State == Failed || item.State == Cancelled {
			delete(m.items, id)
			continue
		}
		newOrder = append(newOrder, id)
	}
	m.order = newOrder
	m.cleanupGroupsLocked()
	m.mu.Unlock()
}

func (m *Manager) ActiveCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, item := range m.items {
		if item.State == Waiting || item.State == Running {
			count++
		}
	}
	return count
}

func (item Item) Name() string {
	if item.Kind == Upload {
		return filepath.Base(item.LocalPath)
	}
	return filepath.Base(item.Key)
}

func (item Item) Progress() float64 {
	if item.Total <= 0 {
		if item.State == Done {
			return 1
		}
		return 0
	}
	value := float64(item.Completed) / float64(item.Total)
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func (item Item) Speed() float64 {
	if item.StartedAt.IsZero() || item.Completed <= 0 {
		return 0
	}
	seconds := time.Since(item.StartedAt).Seconds()
	if seconds <= 0 {
		return 0
	}
	return float64(item.Completed) / seconds
}

func (m *Manager) PruneSuccessful(olderThan time.Duration) {
	cutoff := time.Now().Add(-olderThan)
	m.mu.Lock()
	newOrder := m.order[:0]
	for _, id := range m.order {
		item := m.items[id]
		if item == nil {
			continue
		}
		if (item.State == Done || item.State == Cancelled) && !item.FinishedAt.IsZero() && item.FinishedAt.Before(cutoff) {
			delete(m.items, id)
			continue
		}
		newOrder = append(newOrder, id)
	}
	m.order = newOrder
	m.cleanupGroupsLocked()
	m.mu.Unlock()
}

func (m *Manager) cleanupGroupsLocked() {
	active := make(map[string]struct{})
	for _, item := range m.items {
		if item.GroupID != "" {
			active[item.GroupID] = struct{}{}
		}
	}
	for id, closed := range m.groups {
		if !closed {
			continue
		}
		if _, ok := active[id]; !ok {
			delete(m.groups, id)
		}
	}
}

// Retry queues a failed or cancelled transfer again from the start. It
// reports whether the item was requeued.
func (m *Manager) Retry(id string) bool {
	m.mu.Lock()
	item, ok := m.items[id]
	if !ok || (item.State != Failed && item.State != Cancelled) {
		m.mu.Unlock()
		return false
	}
	item.ctx, item.cancel = context.WithCancel(context.Background())
	item.State = Waiting
	item.Err = nil
	item.Completed = 0
	item.StartedAt = time.Time{}
	item.FinishedAt = time.Time{}
	m.mu.Unlock()
	m.queue <- item
	return true
}
