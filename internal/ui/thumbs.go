package ui

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // decoders for previews
	_ "image/jpeg"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"github.com/fyne-io/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"r2go/internal/model"
	"r2go/internal/r2"
)

const (
	thumbMaxSource int64 = 100 << 20 // previews are only made for images up to 100 MB
	thumbEdge            = 256       // longest side of a cached preview, in pixels
	thumbWorkers         = 2
	thumbMemory          = 400 // previews kept decoded in memory
)

// previewable reports whether r2go can draw a preview for the entry.
func previewable(entry model.Entry) bool {
	if entry.IsDir || entry.Size <= 0 || entry.Size > thumbMaxSource {
		return false
	}
	switch fileExt(entry.Name) {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "svg", "ico":
		return true
	}
	return false
}

// thumbCache makes image previews on demand: only for rows or tiles that are
// on screen, at most three downloads at a time, resized to 256 px and cached
// on disk by ETag so a revisit costs nothing and a changed file refreshes on
// its own. Loads that are still waiting are dropped when the folder changes.
type thumbCache struct {
	dir string
	sem chan struct{}

	mu     sync.Mutex
	items  map[string]*thumbItem
	ctx    context.Context
	cancel context.CancelFunc
}

type thumbItem struct {
	img     image.Image
	failed  bool
	loading bool
	used    time.Time
	waiters []func(image.Image)
}

func newThumbCache() *thumbCache {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	c := &thumbCache{
		dir:   filepath.Join(dir, "r2go", "thumbs"),
		sem:   make(chan struct{}, thumbWorkers),
		items: map[string]*thumbItem{},
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	return c
}

func thumbKey(session model.Session, entry model.Entry) string {
	sum := sha1.Sum([]byte(session.Account + "\x00" + session.Bucket + "\x00" + entry.Key + "\x00" + entry.ETag + "\x00" + strconv.FormatInt(entry.Size, 10)))
	return hex.EncodeToString(sum[:])
}

// Reset drops loads that have not started, after the folder changed.
func (c *thumbCache) Reset() {
	c.mu.Lock()
	c.cancel()
	c.ctx, c.cancel = context.WithCancel(context.Background())
	for key, item := range c.items {
		if item.loading {
			delete(c.items, key)
		}
	}
	c.mu.Unlock()
}

// Get returns a ready preview, or starts making one and calls ready on the UI
// goroutine when it exists. It returns nil while the preview is on its way.
func (c *thumbCache) Get(client *r2.Client, session model.Session, entry model.Entry, ready func(image.Image)) image.Image {
	if !previewable(entry) || client == nil {
		return nil
	}
	key := thumbKey(session, entry)
	c.mu.Lock()
	item, ok := c.items[key]
	if ok {
		item.used = time.Now()
		if item.img != nil || item.failed {
			c.mu.Unlock()
			return item.img
		}
		if ready != nil {
			item.waiters = append(item.waiters, ready)
		}
		c.mu.Unlock()
		return nil
	}
	item = &thumbItem{loading: true, used: time.Now()}
	if ready != nil {
		item.waiters = append(item.waiters, ready)
	}
	c.items[key] = item
	ctx := c.ctx
	c.evictLocked()
	c.mu.Unlock()

	go c.load(ctx, client, session, entry, key)
	return nil
}

func (c *thumbCache) load(ctx context.Context, client *r2.Client, session model.Session, entry model.Entry, key string) {
	img := c.readDisk(key)
	if img == nil {
		select {
		case c.sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		img = c.fetch(ctx, client, session, entry, key)
		<-c.sem
	}
	c.mu.Lock()
	item, ok := c.items[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	item.loading = false
	item.img = img
	item.failed = img == nil
	waiters := item.waiters
	item.waiters = nil
	c.mu.Unlock()
	if img == nil {
		return
	}
	fyne.Do(func() {
		for _, waiter := range waiters {
			waiter(img)
		}
	})
}

func (c *thumbCache) path(key string) string { return filepath.Join(c.dir, key[:2], key+".png") }

func (c *thumbCache) readDisk(key string) image.Image {
	data, err := os.ReadFile(c.path(key))
	if err != nil {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return img
}

func (c *thumbCache) fetch(ctx context.Context, client *r2.Client, session model.Session, entry model.Entry, key string) image.Image {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	data, err := client.ReadObject(ctx, session.Bucket, entry.Key, thumbMaxSource)
	if err != nil {
		return nil
	}
	var img image.Image
	switch fileExt(entry.Name) {
	case "svg":
		img, err = rasterizeSVG(data, thumbEdge)
	case "ico":
		var src image.Image
		if src, err = decodeICO(data); err == nil {
			img = shrink(src, thumbEdge)
		}
	default:
		var src image.Image
		src, _, err = image.Decode(bytes.NewReader(data))
		if err == nil {
			img = shrink(src, thumbEdge)
		}
	}
	if err != nil || img == nil {
		return nil
	}
	var buf bytes.Buffer
	if png.Encode(&buf, img) == nil {
		path := c.path(key)
		if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
			_ = os.WriteFile(path, buf.Bytes(), 0o644)
		}
	}
	return img
}

// shrink scales src so its longest side is at most edge pixels.
func shrink(src image.Image, edge int) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= edge && h <= edge {
		return src
	}
	if w >= h {
		h = max(1, h*edge/w)
		w = edge
	} else {
		w = max(1, w*edge/h)
		h = edge
	}
	// An *image.RGBA destination keeps x/image on its fast paths (a JPEG's
	// YCbCr source included); bilinear is plenty for a 256 px preview and
	// far cheaper than Catmull-Rom on a large photo.
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, bounds, xdraw.Src, nil)
	return dst
}

// rasterizeSVG draws an SVG so its longest side is edge pixels.
func rasterizeSVG(data []byte, edge int) (image.Image, error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.IgnoreErrorMode)
	if err != nil {
		return nil, err
	}
	w, h := icon.ViewBox.W, icon.ViewBox.H
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("svg has no size")
	}
	scale := float64(edge) / math.Max(w, h)
	width, height := max(1, int(w*scale)), max(1, int(h*scale))
	icon.SetTarget(0, 0, float64(width), float64(height))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	scanner := rasterx.NewScannerGV(width, height, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(width, height, scanner), 1)
	return img, nil
}

// evictLocked keeps the in-memory cache bounded, dropping the least recently
// used finished previews.
func (c *thumbCache) evictLocked() {
	for len(c.items) > thumbMemory {
		oldestKey := ""
		var oldest time.Time
		for key, item := range c.items {
			if item.loading {
				continue
			}
			if oldestKey == "" || item.used.Before(oldest) {
				oldestKey, oldest = key, item.used
			}
		}
		if oldestKey == "" {
			return
		}
		delete(c.items, oldestKey)
	}
}

// DiskUsage is the size of the preview cache on disk.
func (c *thumbCache) DiskUsage() int64 {
	var total int64
	_ = filepath.WalkDir(c.dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// Clear deletes every cached preview.
func (c *thumbCache) Clear() error {
	c.mu.Lock()
	c.items = map[string]*thumbItem{}
	c.mu.Unlock()
	return os.RemoveAll(c.dir)
}
