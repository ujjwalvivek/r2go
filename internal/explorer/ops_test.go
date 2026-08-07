package explorer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"r2go/internal/model"
)

type fakeClient struct {
	objects      map[string][]string
	existing     map[string]bool
	copies       [][2]string
	deleted      []string
	deletedBatch [][]string
	copyErrAt    int
}

func (f *fakeClient) ListKeys(_ context.Context, _ string, prefix string) ([]string, error) {
	return append([]string(nil), f.objects[prefix]...), nil
}

func (f *fakeClient) Exists(_ context.Context, _ string, key string) (bool, error) {
	return f.existing[key], nil
}

func (f *fakeClient) CopyObject(_ context.Context, _ string, src, dst string) error {
	if f.copyErrAt > 0 && len(f.copies)+1 == f.copyErrAt {
		return errors.New("copy failed")
	}
	f.copies = append(f.copies, [2]string{src, dst})
	return nil
}

func (f *fakeClient) DeleteObject(_ context.Context, _ string, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func (f *fakeClient) DeleteObjects(_ context.Context, _ string, keys []string) error {
	copyKeys := append([]string(nil), keys...)
	f.deletedBatch = append(f.deletedBatch, copyKeys)
	return nil
}

func TestCopyFolderPreservesRootAndHierarchy(t *testing.T) {
	client := &fakeClient{objects: map[string][]string{
		"photos/": {
			"photos/a.jpg",
			"photos/nested/b.jpg",
		},
	}, existing: map[string]bool{}}
	entry := model.Entry{Key: "photos/", Name: "photos", IsDir: true}

	if err := Copy(context.Background(), client, "bucket", []model.Entry{entry}, "archive/", ReplaceExisting, nil); err != nil {
		t.Fatal(err)
	}
	want := [][2]string{
		{"photos/a.jpg", "archive/photos/a.jpg"},
		{"photos/nested/b.jpg", "archive/photos/nested/b.jpg"},
	}
	if !reflect.DeepEqual(client.copies, want) {
		t.Fatalf("copies = %#v, want %#v", client.copies, want)
	}
}

func TestCopyRejectsSameFolder(t *testing.T) {
	client := &fakeClient{objects: map[string][]string{}, existing: map[string]bool{}}
	entry := model.Entry{Key: "photos/", Name: "photos", IsDir: true}
	if err := Copy(context.Background(), client, "bucket", []model.Entry{entry}, "", ReplaceExisting, nil); err == nil || !strings.Contains(err.Error(), "already in") {
		t.Fatalf("expected same-folder error, got %v", err)
	}
}

func TestCopyRejectsFolderInsideItself(t *testing.T) {
	client := &fakeClient{objects: map[string][]string{}, existing: map[string]bool{}}
	entry := model.Entry{Key: "photos/", Name: "photos", IsDir: true}
	if err := Copy(context.Background(), client, "bucket", []model.Entry{entry}, "photos/archive/", ReplaceExisting, nil); err == nil || !strings.Contains(err.Error(), "inside itself") {
		t.Fatalf("expected recursive destination error, got %v", err)
	}
}

func TestMoveSkipsExistingWithoutDeletingSource(t *testing.T) {
	client := &fakeClient{
		objects:  map[string][]string{},
		existing: map[string]bool{"archive/a.txt": true},
	}
	entry := model.Entry{Key: "a.txt", Name: "a.txt"}
	if err := Move(context.Background(), client, "bucket", []model.Entry{entry}, "archive/", SkipExisting, nil); err != nil {
		t.Fatal(err)
	}
	if len(client.copies) != 0 || len(client.deleted) != 0 {
		t.Fatalf("skip should not mutate: copies=%v deleted=%v", client.copies, client.deleted)
	}
}

func TestRenameFolderRollsBackCreatedCopiesOnFailure(t *testing.T) {
	client := &fakeClient{
		objects: map[string][]string{
			"old/": {"old/a", "old/b"},
		},
		existing:  map[string]bool{},
		copyErrAt: 2,
	}
	entry := model.Entry{Key: "old/", Name: "old", IsDir: true}
	if err := Rename(context.Background(), client, "bucket", entry, "new", nil); err == nil {
		t.Fatal("expected rename failure")
	}
	if len(client.deletedBatch) != 1 || !reflect.DeepEqual(client.deletedBatch[0], []string{"new/a"}) {
		t.Fatalf("rollback deletes = %#v", client.deletedBatch)
	}
}

func TestRenameRejectsPathSeparators(t *testing.T) {
	client := &fakeClient{objects: map[string][]string{}, existing: map[string]bool{}}
	entry := model.Entry{Key: "a.txt", Name: "a.txt"}
	for _, name := range []string{"folder/a.txt", `folder\\a.txt`} {
		if err := Rename(context.Background(), client, "bucket", entry, name, nil); err == nil || !strings.Contains(err.Error(), "path separators") {
			t.Fatalf("Rename(%q) expected separator error, got %v", name, err)
		}
	}
}

func TestDeleteBatchesAtOneThousandObjects(t *testing.T) {
	keys := make([]string, 2005)
	for i := range keys {
		keys[i] = fmt.Sprintf("folder/%04d", i)
	}
	client := &fakeClient{
		objects:  map[string][]string{"folder/": keys},
		existing: map[string]bool{},
	}
	entry := model.Entry{Key: "folder/", Name: "folder", IsDir: true}
	if err := Delete(context.Background(), client, "bucket", []model.Entry{entry}, nil); err != nil {
		t.Fatal(err)
	}
	if got := len(client.deletedBatch); got != 3 {
		t.Fatalf("delete batches = %d, want 3", got)
	}
	want := []int{1000, 1000, 5}
	for i, size := range want {
		if got := len(client.deletedBatch[i]); got != size {
			t.Fatalf("batch %d size = %d, want %d", i, got, size)
		}
	}
}
