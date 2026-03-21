package mount

import "strings"

// PathToKey maps a WinFsp/FUSE absolute path to the R2 object key under a root prefix.
func PathToKey(rootPrefix, path string) string {
	p := strings.TrimPrefix(path, "/")
	if p == "" {
		return strings.TrimSuffix(rootPrefix, "/")
	}
	return strings.TrimSuffix(rootPrefix, "/") + "/" + p
}

// KeyToPath maps an R2 key back to an absolute mount path.
func KeyToPath(rootPrefix, key string) string {
	root := strings.TrimSuffix(rootPrefix, "/")
	if root == "" {
		return "/" + key
	}
	return "/" + strings.TrimPrefix(key, root+"/")
}

// DirKey returns the R2 marker key for a directory path.
func DirKey(rootPrefix, path string) string {
	k := PathToKey(rootPrefix, path)
	if k == "" {
		return ""
	}
	return strings.TrimSuffix(k, "/") + "/"
}

// Base returns the last path segment.
func Base(path string) string {
	path = strings.TrimSuffix(path, "/")
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// ParentDirKey returns the directory key that contains the given key.
func ParentDirKey(rootPrefix, key string) string {
	dir := "/" + key
	i := strings.LastIndex(dir, "/")
	dir = dir[:i+1]
	return DirKey(rootPrefix, dir)
}
