package transfer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"r2go/internal/s3client"
)

type Kind int

const (
	Upload Kind = iota
	Download
)

func (k Kind) String() string {
	if k == Upload {
		return "Upload"
	}
	return "Download"
}

type Status int

const (
	Queued Status = iota
	Running
	Paused
	Done
	Failed
	Cancelled
)

func (s Status) String() string {
	switch s {
	case Queued:
		return "Queued"
	case Running:
		return "Running"
	case Paused:
		return "Paused"
	case Done:
		return "Done"
	case Failed:
		return "Failed"
	case Cancelled:
		return "Cancelled"
	}
	return "?"
}

type Item struct {
	ID       int64
	Kind     Kind
	Status   Status
	Account  string
	Bucket   string
	Key      string
	Local    string
	Size     int64
	Done     int64
	Speed    float64
	Err      error
	AutoOpen bool

	cancel  context.CancelFunc
	pause   bool
	lastT   time.Time
	lastN   int64
}

type Manager struct {
	mu       sync.Mutex
	items    []*Item
	nextID   int64
	sem      chan struct{}
	onChange func()
	onFail   func(*Item)
	lastFire time.Time
}

func New(concurrency int, onChange func()) *Manager {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Manager{
		sem:      make(chan struct{}, concurrency),
		onChange: onChange,
	}
}

func (m *Manager) OnFail(cb func(*Item)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onFail = cb
}

func (m *Manager) SetConcurrency(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n <= 0 {
		n = 4
	}
	old := cap(m.sem)
	if n == old {
		return
	}
	m.sem = make(chan struct{}, n)
}

func (m *Manager) fire() {
	if m.onChange == nil {
		return
	}
	m.mu.Lock()
	now := time.Now()
	if now.Sub(m.lastFire) < 150*time.Millisecond {
		m.mu.Unlock()
		return
	}
	m.lastFire = now
	m.mu.Unlock()
	m.onChange()
}

func (m *Manager) Items() []*Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Item, len(m.items))
	for i, it := range m.items {
		c := *it
		out[i] = &c
	}
	return out
}

func (m *Manager) Find(id int64) *Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range m.items {
		if it.ID == id {
			c := *it
			return &c
		}
	}
	return nil
}

func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, it := range m.items {
		if it.Status == Queued || it.Status == Running {
			n++
		}
	}
	return n
}

func (m *Manager) AddUpload(client *s3client.Client, account, bucket, key, local string) *Item {
	it := m.add(client, Upload, account, bucket, key, local)
	go m.run(client, it)
	return it
}

func (m *Manager) AddDownload(client *s3client.Client, account, bucket, key, dest string) *Item {
	it := m.add(client, Download, account, bucket, key, dest)
	go m.run(client, it)
	return it
}

func (m *Manager) add(client *s3client.Client, kind Kind, account, bucket, key, local string) *Item {
	m.mu.Lock()
	m.nextID++
	it := &Item{
		ID:      m.nextID,
		Kind:    kind,
		Status:  Queued,
		Account: account,
		Bucket:  bucket,
		Key:     key,
		Local:   local,
		lastT:   time.Now(),
	}
	if kind == Download {
		if st, err := os.Stat(local); err == nil {
			it.Done = st.Size()
			it.Size = -1
		}
	}
	m.items = append(m.items, it)
	m.mu.Unlock()
	m.fire()
	return it
}

func (m *Manager) run(client *s3client.Client, it *Item) {
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	it.cancel = cancel
	it.Status = Running
	it.lastT = time.Now()
	it.lastN = it.Done
	it.pause = false
	m.mu.Unlock()
	m.fire()

	var err error
	if it.Kind == Upload {
		err = m.runUpload(ctx, client, it)
	} else {
		err = m.runDownload(ctx, client, it)
	}

	m.mu.Lock()
	if err == nil {
		it.Status = Done
	} else if err == context.Canceled || err == context.DeadlineExceeded {
		if it.pause {
			it.Status = Paused
		} else {
			it.Status = Cancelled
		}
	} else {
		it.Status = Failed
		it.Err = err
	}
	it.cancel = nil
	fail := it.Status == Failed && m.onFail != nil
	m.mu.Unlock()
	m.fire()
	if fail {
		c := *it
		m.onFail(&c)
	}
}

func (m *Manager) runUpload(ctx context.Context, client *s3client.Client, it *Item) error {
	f, err := os.Open(it.Local)
	if err != nil {
		return err
	}
	defer f.Close()

	if it.Size <= 0 {
		if st, serr := f.Stat(); serr == nil {
			it.Size = st.Size()
		}
	}
	return client.Upload(ctx, it.Bucket, it.Key, f, it.Size, func(n int64) {
		m.tick(it, n)
	})
}

func (m *Manager) runDownload(ctx context.Context, client *s3client.Client, it *Item) error {
	offset := it.Done
	if offset > 0 {
		st, err := os.Stat(it.Local)
		if err == nil && st.Size() == offset {
			offset = st.Size()
		} else {
			offset = 0
			_ = os.Remove(it.Local)
		}
	}
	dest, err := os.OpenFile(it.Local, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer dest.Close()

	if it.Size <= 0 {
		if h, herr := client.HeadObject(ctx, it.Bucket, it.Key); herr == nil && h.ContentLength != nil {
			it.Size = *h.ContentLength
		}
	}
	return client.Download(ctx, it.Bucket, it.Key, dest, offset, func(n int64) {
		m.tick(it, n)
	})
}

func (m *Manager) tick(it *Item, done int64) {
	m.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(it.lastT).Seconds()
	if elapsed >= 0.5 {
		delta := done - it.lastN
		it.Speed = float64(delta) / elapsed
		it.lastT = now
		it.lastN = done
	}
	it.Done = done
	m.mu.Unlock()
	m.fire()
}

func (m *Manager) Pause(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range m.items {
		if it.ID == id && it.Status == Running {
			it.pause = true
			if it.cancel != nil {
				it.cancel()
			}
			return
		}
	}
}

func (m *Manager) Cancel(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range m.items {
		if it.ID == id {
			it.pause = false
			if it.cancel != nil {
				it.cancel()
			} else if it.Status == Queued {
				it.Status = Cancelled
				m.fire()
			}
			return
		}
	}
}

func (m *Manager) Resume(client *s3client.Client, id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range m.items {
		if it.ID == id && (it.Status == Paused || it.Status == Cancelled) && it.Kind == Download {
			it.Status = Queued
			it.Err = nil
			it.pause = false
			go m.run(client, it)
			return
		}
	}
}

func (m *Manager) Dismiss(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, it := range m.items {
		if it.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			m.fire()
			return
		}
	}
}

func (it *Item) Progress() float64 {
	if it.Size <= 0 {
		return 0
	}
	if it.Done >= it.Size {
		return 1
	}
	return float64(it.Done) / float64(it.Size)
}

func (it *Item) Detail() string {
	prog := fmt.Sprintf("%.1f%%", it.Progress()*100)
	if it.Size > 0 && it.Done > 0 {
		prog = fmt.Sprintf("%s of %s", humanBytes(it.Done), humanBytes(it.Size))
	}
	switch it.Status {
	case Queued:
		return "Queued"
	case Running:
		if it.Speed > 0 {
			return fmt.Sprintf("%s · %s/s", prog, humanBytes(int64(it.Speed)))
		}
		return prog
	case Paused:
		return "Paused — " + prog
	case Done:
		return humanBytes(it.Size)
	case Cancelled:
		return "Cancelled"
	case Failed:
		if it.Err != nil {
			return "Error: " + it.Err.Error()
		}
		return "Failed"
	}
	return ""
}

func humanBytes(n int64) string {
	const kb = 1024
	switch {
	case n >= kb*kb*kb:
		return fmt.Sprintf("%.1f GB", float64(n)/(kb*kb*kb))
	case n >= kb*kb:
		return fmt.Sprintf("%.1f MB", float64(n)/(kb*kb))
	case n >= kb:
		return fmt.Sprintf("%.1f KB", float64(n)/kb)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
