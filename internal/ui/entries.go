package ui

import (
	"sort"
	"strings"
	"time"
)

type Entry struct {
	Key   string
	Name  string
	IsDir bool
	Size  int64
	Mod   time.Time
	ETag  string
}

const (
	SortName = iota
	SortSize
	SortMod
)

type sortState struct {
	field int
	desc  bool
}

func sortEntries(es []Entry, s sortState) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		c := compareEntry(a, b, s.field)
		if s.desc && s.field != SortName {
			return c > 0
		}
		return c < 0
	})
}

func compareEntry(a, b Entry, field int) int {
	switch field {
	case SortSize:
		if a.Size != b.Size {
			return int(a.Size - b.Size)
		}
	case SortMod:
		if !a.Mod.Equal(b.Mod) {
			if a.Mod.Before(b.Mod) {
				return -1
			}
			return 1
		}
	}
	return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
}

func matchesFilter(e Entry, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.Name), strings.ToLower(filter))
}
