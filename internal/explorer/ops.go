package explorer

import (
	"context"
	"fmt"
	"strings"

	"r2go/internal/model"
)

type ConflictPolicy int

const (
	SkipExisting ConflictPolicy = iota
	ReplaceExisting
)

type Progress func(done, total int, current string)

// ObjectClient is the small R2 data-plane surface explorer operations need.
// Keeping it as an interface makes destructive operations unit-testable without
// talking to a real bucket.
type ObjectClient interface {
	ListKeys(ctx context.Context, bucket, prefix string) ([]string, error)
	Exists(ctx context.Context, bucket, key string) (bool, error)
	CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error
	DeleteObject(ctx context.Context, bucket, key string) error
	DeleteObjects(ctx context.Context, bucket string, keys []string) error
}

type objectMove struct {
	source string
	target string
}

func Delete(ctx context.Context, client ObjectClient, bucket string, entries []model.Entry, progress Progress) error {
	keys, err := expandKeys(ctx, client, bucket, entries)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	const chunkSize = 1000
	for start := 0; start < len(keys); start += chunkSize {
		end := start + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		if err := client.DeleteObjects(ctx, bucket, keys[start:end]); err != nil {
			return err
		}
		if progress != nil {
			progress(end, len(keys), keys[end-1])
		}
	}
	return nil
}

func Copy(ctx context.Context, client ObjectClient, bucket string, entries []model.Entry, destinationPrefix string, policy ConflictPolicy, progress Progress) error {
	moves, err := buildMoves(ctx, client, bucket, entries, destinationPrefix)
	if err != nil {
		return err
	}
	for i, move := range moves {
		exists, err := client.Exists(ctx, bucket, move.target)
		if err != nil {
			return err
		}
		if exists && policy == SkipExisting {
			if progress != nil {
				progress(i+1, len(moves), move.target+" (skipped)")
			}
			continue
		}
		if err := client.CopyObject(ctx, bucket, move.source, move.target); err != nil {
			return err
		}
		if progress != nil {
			progress(i+1, len(moves), move.target)
		}
	}
	return nil
}

func Move(ctx context.Context, client ObjectClient, bucket string, entries []model.Entry, destinationPrefix string, policy ConflictPolicy, progress Progress) error {
	moves, err := buildMoves(ctx, client, bucket, entries, destinationPrefix)
	if err != nil {
		return err
	}
	for i, move := range moves {
		exists, err := client.Exists(ctx, bucket, move.target)
		if err != nil {
			return err
		}
		if exists && policy == SkipExisting {
			if progress != nil {
				progress(i+1, len(moves), move.target+" (skipped)")
			}
			continue
		}
		if err := client.CopyObject(ctx, bucket, move.source, move.target); err != nil {
			return err
		}
		if err := client.DeleteObject(ctx, bucket, move.source); err != nil {
			return fmt.Errorf("copied %s but could not remove source: %w", move.source, err)
		}
		if progress != nil {
			progress(i+1, len(moves), move.target)
		}
	}
	return nil
}

func Rename(ctx context.Context, client ObjectClient, bucket string, entry model.Entry, newName string, progress Progress) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(newName, "/\\") {
		return fmt.Errorf("name must not contain path separators")
	}
	parent := strings.TrimSuffix(entry.Key, entry.Name)
	if entry.IsDir {
		parent = strings.TrimSuffix(entry.Key, entry.Name+"/")
	}
	target := model.JoinPrefix(parent, newName, entry.IsDir)
	if target == entry.Key {
		return nil
	}

	if !entry.IsDir {
		exists, err := client.Exists(ctx, bucket, target)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("%q already exists", newName)
		}
		if err := client.CopyObject(ctx, bucket, entry.Key, target); err != nil {
			return err
		}
		if err := client.DeleteObject(ctx, bucket, entry.Key); err != nil {
			return fmt.Errorf("renamed copy was created, but source could not be removed: %w", err)
		}
		if progress != nil {
			progress(1, 1, target)
		}
		return nil
	}

	keys, err := client.ListKeys(ctx, bucket, entry.Key)
	if err != nil {
		return err
	}
	moves := make([]objectMove, 0, len(keys))
	for _, key := range keys {
		relative := strings.TrimPrefix(key, entry.Key)
		destination := target + relative
		exists, err := client.Exists(ctx, bucket, destination)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("cannot rename: %q already exists", destination)
		}
		moves = append(moves, objectMove{source: key, target: destination})
	}

	created := make([]string, 0, len(moves))
	for i, move := range moves {
		if err := client.CopyObject(ctx, bucket, move.source, move.target); err != nil {
			_ = client.DeleteObjects(context.Background(), bucket, created)
			return err
		}
		created = append(created, move.target)
		if progress != nil {
			progress(i+1, len(moves), move.target)
		}
	}
	sourceKeys := make([]string, 0, len(moves))
	for _, move := range moves {
		sourceKeys = append(sourceKeys, move.source)
	}
	if err := client.DeleteObjects(ctx, bucket, sourceKeys); err != nil {
		return fmt.Errorf("rename copied successfully but source cleanup failed: %w", err)
	}
	return nil
}

func buildMoves(ctx context.Context, client ObjectClient, bucket string, entries []model.Entry, destinationPrefix string) ([]objectMove, error) {
	destinationPrefix = strings.TrimPrefix(destinationPrefix, "/")
	if destinationPrefix != "" && !strings.HasSuffix(destinationPrefix, "/") {
		destinationPrefix += "/"
	}
	var moves []objectMove
	for _, entry := range entries {
		if entry.IsDir {
			root := destinationPrefix + entry.Name + "/"
			if root == entry.Key {
				return nil, fmt.Errorf("%q is already in that folder", entry.Name)
			}
			if strings.HasPrefix(root, entry.Key) {
				return nil, fmt.Errorf("cannot copy or move %q inside itself", entry.Name)
			}
			keys, err := client.ListKeys(ctx, bucket, entry.Key)
			if err != nil {
				return nil, err
			}
			for _, key := range keys {
				relative := strings.TrimPrefix(key, entry.Key)
				moves = append(moves, objectMove{source: key, target: root + relative})
			}
			continue
		}
		target := destinationPrefix + entry.Name
		if target == entry.Key {
			return nil, fmt.Errorf("%q is already in that folder", entry.Name)
		}
		moves = append(moves, objectMove{source: entry.Key, target: target})
	}
	return moves, nil
}

func expandKeys(ctx context.Context, client ObjectClient, bucket string, entries []model.Entry) ([]string, error) {
	seen := map[string]struct{}{}
	var keys []string
	add := func(key string) {
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for _, entry := range entries {
		if !entry.IsDir {
			add(entry.Key)
			continue
		}
		keys, err := client.ListKeys(ctx, bucket, entry.Key)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			add(key)
		}
	}
	return keys, nil
}
