package model

import (
	"sort"
	"strings"
)

func FilterAndSort(entries []Entry, filter string, state SortState) []Entry {
	filter = strings.ToLower(strings.TrimSpace(filter))
	out := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if filter == "" || strings.Contains(strings.ToLower(entry.Name), filter) {
			out = append(out, entry)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}

		cmp := compare(a, b, state.Field)
		if state.Desc {
			return cmp > 0
		}
		return cmp < 0
	})
	return out
}

func compare(a, b Entry, field SortField) int {
	switch field {
	case SortSize:
		if a.Size < b.Size {
			return -1
		}
		if a.Size > b.Size {
			return 1
		}
	case SortModified:
		if a.Modified.Before(b.Modified) {
			return -1
		}
		if a.Modified.After(b.Modified) {
			return 1
		}
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func ParentPrefix(prefix string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return ""
	}
	idx := strings.LastIndex(prefix, "/")
	if idx < 0 {
		return ""
	}
	return prefix[:idx+1]
}

func JoinPrefix(prefix, name string, dir bool) string {
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	name = strings.Trim(name, "/")
	key := prefix + name
	if dir && key != "" {
		key += "/"
	}
	return key
}
