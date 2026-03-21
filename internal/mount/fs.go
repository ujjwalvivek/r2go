package mount

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/winfsp/cgofuse/fuse"

	"r2go/internal/s3client"
)

// FS is a WinFsp/FUSE filesystem backed by an R2 bucket.
type FS struct {
	fuse.FileSystemBase

	client   *s3client.Client
	bucket   string
	root     string
	readOnly bool
	cacheDir string

	mu      sync.Mutex
	handles map[uint64]*handle
	nextFH  uint64
}

type handle struct {
	path  string
	key   string
	dirty bool
	file  *os.File // write cache file
	size  int64
}

func NewFS(client *s3client.Client, bucket, root string, readOnly bool) *FS {
	cache, _ := os.UserCacheDir()
	return &FS{
		client:   client,
		bucket:   bucket,
		root:     root,
		readOnly: readOnly,
		cacheDir: filepath.Join(cache, "r2go", "mounts", bucket),
		handles:  map[uint64]*handle{},
	}
}

func (fs *FS) cachePath(path string) string {
	return filepath.Join(fs.cacheDir, filepath.FromSlash(path))
}

func (fs *FS) openHandle(path string, truncate bool) (uint64, *handle, error) {
	key := PathToKey(fs.root, path)
	cache := fs.cachePath(path)
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return ^uint64(0), nil, err
	}
	flags := os.O_CREATE | os.O_RDWR
	if truncate {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(cache, flags, 0o644)
	if err != nil {
		return ^uint64(0), nil, err
	}
	h := &handle{path: path, key: key, file: f, dirty: truncate}
	if st, err := f.Stat(); err == nil {
		h.size = st.Size()
	}
	fs.mu.Lock()
	fs.nextFH++
	fh := fs.nextFH
	fs.handles[fh] = h
	fs.mu.Unlock()
	return fh, h, nil
}

func (fs *FS) lookup(fh uint64) *handle {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.handles[fh]
}

func (fs *FS) drop(fh uint64) {
	fs.mu.Lock()
	delete(fs.handles, fh)
	fs.mu.Unlock()
}

func (fs *FS) getattrFor(stat *fuse.Stat_t, isDir bool, size int64, mod time.Time) {
	stat.Mode = fuse.S_IFREG | 0o644
	stat.Nlink = 1
	stat.Size = size
	stat.Blksize = 4096
	stat.Blocks = (size + 511) / 512
	if isDir {
		stat.Mode = fuse.S_IFDIR | 0o755
		stat.Nlink = 2
		stat.Size = 0
		stat.Blocks = 0
	}
	if !mod.IsZero() {
		stat.Atim = fuse.NewTimespec(mod)
		stat.Mtim = fuse.NewTimespec(mod)
		stat.Ctim = fuse.NewTimespec(mod)
	}
}

func (fs *FS) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	if path == "/" {
		fs.getattrFor(stat, true, 0, time.Time{})
		return 0
	}
	if fh != ^uint64(0) {
		if h := fs.lookup(fh); h != nil {
			fs.getattrFor(stat, false, h.size, time.Time{})
			return 0
		}
	}

	key := PathToKey(fs.root, path)
	ctx := context.Background()
	page, err := fs.client.ListPage(ctx, fs.bucket, key, "")
	if err != nil {
		return -fuse.EIO
	}
	for _, o := range page.Objects {
		if o.Key == key {
			fs.getattrFor(stat, false, o.Size, o.ModTime)
			return 0
		}
	}
	if len(page.Prefixes) > 0 || len(page.Objects) > 0 {
		fs.getattrFor(stat, true, 0, time.Time{})
		return 0
	}
	// folder marker or content in deeper pages
	page2, err2 := fs.client.ListPage(ctx, fs.bucket, key+"/", "")
	if err2 != nil {
		return -fuse.EIO
	}
	if len(page2.Prefixes) > 0 || len(page2.Objects) > 0 {
		fs.getattrFor(stat, true, 0, time.Time{})
		return 0
	}
	return -fuse.ENOENT
}

func (fs *FS) ReadDir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) int {
	prefix := PathToKey(fs.root, path)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	ctx := context.Background()
	fill(".", nil, 0)
	fill("..", nil, 0)
	token := ""
	for {
		page, err := fs.client.ListPage(ctx, fs.bucket, prefix, token)
		if err != nil {
			return -fuse.EIO
		}
		for _, p := range page.Prefixes {
			name := strings.TrimSuffix(strings.TrimPrefix(p.Prefix, prefix), "/")
			if name != "" && !fill(name, nil, 0) {
				return 0
			}
		}
		for _, o := range page.Objects {
			name := strings.TrimPrefix(o.Key, prefix)
			if name != "" && !fill(name, nil, 0) {
				return 0
			}
		}
		if !page.Truncated {
			return 0
		}
		token = page.NextToken
	}
}

func (fs *FS) Open(path string, flags int) (int, uint64) {
	if path == "/" {
		return -fuse.EISDIR, ^uint64(0)
	}
	writable := (flags & fuse.O_ACCMODE) != fuse.O_RDONLY
	if fs.readOnly && writable {
		return -fuse.EROFS, ^uint64(0)
	}
	if !writable {
		key := PathToKey(fs.root, path)
		page, err := fs.client.ListPage(context.Background(), fs.bucket, key, "")
		if err != nil {
			return -fuse.EIO, ^uint64(0)
		}
		found := false
		for _, o := range page.Objects {
			if o.Key == key {
				found = true
				break
			}
		}
		if !found {
			return -fuse.ENOENT, ^uint64(0)
		}
	}
	fh, _, err := fs.openHandle(path, writable)
	if err != nil {
		return -fuse.EIO, ^uint64(0)
	}
	return 0, fh
}

func (fs *FS) Read(path string, buff []byte, ofst int64, fh uint64) int {
	h := fs.lookup(fh)
	if h == nil {
		return -fuse.EBADF
	}
	if h.file != nil && !h.dirty {
		n, err := h.file.ReadAt(buff, ofst)
		if err == io.EOF && n > 0 {
			return n
		}
		if err != nil && err != io.EOF {
			return -fuse.EIO
		}
		return n
	}
	out, err := fs.client.GetRange(context.Background(), fs.bucket, h.key, ofst, ofst+int64(len(buff))-1)
	if err != nil {
		return -fuse.EIO
	}
	n, rerr := out.Read(buff)
	if rerr != nil && rerr != io.EOF {
		return -fuse.EIO
	}
	return n
}

func (fs *FS) Write(path string, buff []byte, ofst int64, fh uint64) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	h := fs.lookup(fh)
	if h == nil || h.file == nil {
		return -fuse.EBADF
	}
	n, err := h.file.WriteAt(buff, ofst)
	if err != nil {
		return -fuse.EIO
	}
	if ofst+int64(n) > h.size {
		h.size = ofst + int64(n)
	}
	h.dirty = true
	return n
}

func (fs *FS) uploadHandle(h *handle) error {
	f, err := os.Open(fs.cachePath(h.path))
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	return fs.client.Upload(context.Background(), fs.bucket, h.key, f, st.Size(), nil)
}

func (fs *FS) Release(path string, fh uint64) int {
	h := fs.lookup(fh)
	if h == nil {
		return 0
	}
	defer fs.drop(fh)
	if h.file != nil {
		h.file.Close()
	}
	if !h.dirty {
		_ = os.Remove(fs.cachePath(path))
		return 0
	}
	if fs.readOnly {
		return -fuse.EROFS
	}
	if err := fs.uploadHandle(h); err != nil {
		return -fuse.EIO
	}
	_ = os.Remove(fs.cachePath(path))
	return 0
}

func (fs *FS) Truncate(path string, size int64, fh uint64) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	if fh != ^uint64(0) {
		h := fs.lookup(fh)
		if h == nil || h.file == nil {
			return -fuse.EBADF
		}
		if err := h.file.Truncate(size); err != nil {
			return -fuse.EIO
		}
		h.size = size
		h.dirty = true
		return 0
	}
	fh2, h, err := fs.openHandle(path, true)
	if err != nil {
		return -fuse.EIO
	}
	defer fs.drop(fh2)
	defer h.file.Close()
	if err := h.file.Truncate(size); err != nil {
		return -fuse.EIO
	}
	h.size = size
	if err := fs.uploadHandle(h); err != nil {
		return -fuse.EIO
	}
	_ = os.Remove(fs.cachePath(path))
	return 0
}

func (fs *FS) Create(path string, flags int, mode uint32) (int, uint64) {
	if fs.readOnly {
		return -fuse.EROFS, ^uint64(0)
	}
	return fs.Open(path, flags|fuse.O_WRONLY|fuse.O_CREAT|fuse.O_TRUNC)
}

func (fs *FS) Mkdir(path string, mode uint32) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	key := DirKey(fs.root, path)
	if key == "" {
		return -fuse.EEXIST
	}
	if err := fs.client.PutObject(context.Background(), fs.bucket, key, strings.NewReader(""), 0, "", nil); err != nil {
		return -fuse.EIO
	}
	return 0
}

func (fs *FS) Unlink(path string) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	key := PathToKey(fs.root, path)
	if err := fs.client.DeleteObject(context.Background(), fs.bucket, key); err != nil {
		return -fuse.EIO
	}
	return 0
}

func (fs *FS) Rmdir(path string) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	if path == "/" {
		return -fuse.EBUSY
	}
	key := DirKey(fs.root, path)
	objs, err := fs.client.ListAll(context.Background(), fs.bucket, key)
	if err != nil {
		return -fuse.EIO
	}
	if len(objs) > 0 {
		return -fuse.ENOTEMPTY
	}
	_ = fs.client.DeleteObject(context.Background(), fs.bucket, key)
	return 0
}

func (fs *FS) Rename(oldPath, newPath string) int {
	if fs.readOnly {
		return -fuse.EROFS
	}
	if oldPath == "/" || newPath == "/" {
		return -fuse.EINVAL
	}
	oldKey := PathToKey(fs.root, oldPath)
	newKey := PathToKey(fs.root, newPath)

	// destination may be an existing directory → move inside it
	page, err := fs.client.ListPage(context.Background(), fs.bucket, newKey, "")
	if err != nil {
		return -fuse.EIO
	}
	if len(page.Prefixes) > 0 || len(page.Objects) > 0 {
		if !strings.HasSuffix(newKey, "/") {
			newKey += "/"
		}
		newKey += Base(oldPath)
	}

	if !strings.HasSuffix(oldKey, "/") {
		if cerr := fs.client.CopyObject(context.Background(), fs.bucket, oldKey, newKey); cerr != nil {
			return -fuse.EIO
		}
		if derr := fs.client.DeleteObject(context.Background(), fs.bucket, oldKey); derr != nil {
			return -fuse.EIO
		}
		return 0
	}

	oldPrefix := oldKey
	objs, err := fs.client.ListAll(context.Background(), fs.bucket, oldPrefix)
	if err != nil {
		return -fuse.EIO
	}
	newPrefix := strings.TrimSuffix(newKey, "/") + "/"
	for _, o := range objs {
		dst := newPrefix + strings.TrimPrefix(o.Key, oldPrefix)
		if err := fs.client.CopyObject(context.Background(), fs.bucket, o.Key, dst); err != nil {
			return -fuse.EIO
		}
	}
	if len(objs) > 0 {
		keys := make([]string, 0, len(objs))
		for _, o := range objs {
			keys = append(keys, o.Key)
		}
		if err := fs.client.DeleteObjects(context.Background(), fs.bucket, keys); err != nil {
			return -fuse.EIO
		}
	}
	return 0
}

func (fs *FS) Statfs(path string, stat *fuse.Statfs_t) int {
	stat.Frsize = 4096
	stat.Bsize = 4096
	stat.Blocks = 1 << 30
	stat.Bfree = 1 << 30
	stat.Bavail = 1 << 30
	stat.Files = 1 << 20
	stat.Ffree = 1 << 20
	return 0
}

func (fs *FS) Flush(path string, fh uint64) int  { return 0 }
func (fs *FS) Fsync(path string, datasync bool, fh uint64) int { return 0 }
func (fs *FS) Access(path string, mask uint32) int { return 0 }
func (fs *FS) Chmod(path string, mode uint32) int { return 0 }
func (fs *FS) Chown(path string, uid uint32, gid uint32) int { return 0 }
func (fs *FS) Utimens(path string, tmsp []fuse.Timespec) int { return 0 }
