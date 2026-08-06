package model

import (
	"testing"
	"time"
)

func TestFilterAndSortKeepsDirectoriesFirst(t *testing.T) {
	entries := []Entry{
		{Key: "z.txt", Name: "z.txt", Size: 1},
		{Key: "beta/", Name: "beta", IsDir: true},
		{Key: "a.txt", Name: "a.txt", Size: 2},
		{Key: "alpha/", Name: "alpha", IsDir: true},
	}

	got := FilterAndSort(entries, "", SortState{Field: SortName, Desc: true})
	want := []string{"beta", "alpha", "z.txt", "a.txt"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i].Name != name {
			t.Fatalf("row %d = %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestFilterAndSortIsCaseInsensitive(t *testing.T) {
	entries := []Entry{
		{Key: "One.JSON", Name: "One.JSON"},
		{Key: "two.txt", Name: "two.txt"},
	}
	got := FilterAndSort(entries, "json", SortState{Field: SortName})
	if len(got) != 1 || got[0].Name != "One.JSON" {
		t.Fatalf("unexpected filter result: %#v", got)
	}
}

func TestSortModified(t *testing.T) {
	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := old.Add(time.Hour)
	entries := []Entry{
		{Key: "new", Name: "new", Modified: newer},
		{Key: "old", Name: "old", Modified: old},
	}
	got := FilterAndSort(entries, "", SortState{Field: SortModified})
	if got[0].Name != "old" || got[1].Name != "new" {
		t.Fatalf("unexpected modified sort: %#v", got)
	}
}

func TestPrefixHelpers(t *testing.T) {
	tests := []struct {
		prefix string
		parent string
	}{
		{"", ""},
		{"assets/", ""},
		{"assets/images/", "assets/"},
		{"a/b/c/", "a/b/"},
	}
	for _, tc := range tests {
		if got := ParentPrefix(tc.prefix); got != tc.parent {
			t.Fatalf("ParentPrefix(%q) = %q, want %q", tc.prefix, got, tc.parent)
		}
	}

	if got := JoinPrefix("assets/", "images", true); got != "assets/images/" {
		t.Fatalf("JoinPrefix dir = %q", got)
	}
	if got := JoinPrefix("assets", "/logo.png/", false); got != "assets/logo.png" {
		t.Fatalf("JoinPrefix file = %q", got)
	}
}
