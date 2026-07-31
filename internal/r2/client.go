package r2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"r2go/internal/config"
)

type Client struct {
	Account config.Account
	svc     *s3.Client
}

func New(account config.Account, secret string) *Client {
	cfg := aws.Config{
		Region:      "auto",
		Credentials: credentials.NewStaticCredentialsProvider(account.AccessKeyID, secret, ""),
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", account.AccountID)
	svc := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	return &Client{Account: account, svc: svc}
}

type Object struct {
	Key      string
	Size     int64
	Modified time.Time
	ETag     string
}

type ListPage struct {
	Objects   []Object
	Prefixes  []string
	NextToken string
	Truncated bool
}

func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	out, err := c.svc.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	buckets := make([]string, 0, len(out.Buckets))
	for _, bucket := range out.Buckets {
		if bucket.Name != nil {
			buckets = append(buckets, *bucket.Name)
		}
	}
	return buckets, nil
}

func (c *Client) ListPage(ctx context.Context, bucket, prefix, token string) (ListPage, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	if token != "" {
		input.ContinuationToken = aws.String(token)
	}
	out, err := c.svc.ListObjectsV2(ctx, input)
	if err != nil {
		return ListPage{}, fmt.Errorf("list %s/%s: %w", bucket, prefix, err)
	}
	page := ListPage{Truncated: aws.ToBool(out.IsTruncated)}
	if out.NextContinuationToken != nil {
		page.NextToken = *out.NextContinuationToken
	}
	for _, prefix := range out.CommonPrefixes {
		if prefix.Prefix != nil {
			page.Prefixes = append(page.Prefixes, *prefix.Prefix)
		}
	}
	for _, object := range out.Contents {
		if object.Key == nil || *object.Key == prefix {
			continue
		}
		item := Object{Key: *object.Key, Size: aws.ToInt64(object.Size), ETag: aws.ToString(object.ETag)}
		if object.LastModified != nil {
			item.Modified = *object.LastModified
		}
		page.Objects = append(page.Objects, item)
	}
	return page, nil
}

func (c *Client) ListAll(ctx context.Context, bucket, prefix string) ([]Object, error) {
	var objects []Object
	var token *string
	for {
		out, err := c.svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list recursively %s/%s: %w", bucket, prefix, err)
		}
		for _, object := range out.Contents {
			if object.Key == nil {
				continue
			}
			item := Object{Key: *object.Key, Size: aws.ToInt64(object.Size), ETag: aws.ToString(object.ETag)}
			if object.LastModified != nil {
				item.Modified = *object.LastModified
			}
			objects = append(objects, item)
		}
		if !aws.ToBool(out.IsTruncated) {
			return objects, nil
		}
		token = out.NextContinuationToken
	}
}

func (c *Client) ListKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	objects, err := c.ListAll(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(objects))
	for _, object := range objects {
		keys = append(keys, object.Key)
	}
	return keys, nil
}

func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := c.svc.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	var responseErr *smithyhttp.ResponseError
	if errors.As(err, &responseErr) && responseErr.HTTPStatusCode() == 404 {
		return false, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchObject", "404":
			return false, nil
		}
	}
	return false, fmt.Errorf("check %s: %w", key, err)
}

func (c *Client) CreateFolder(ctx context.Context, bucket, key string) error {
	_, err := c.svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(nil),
	})
	if err != nil {
		return fmt.Errorf("create folder %s: %w", key, err)
	}
	return nil
}

func (c *Client) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	copySource := encodeCopySource(bucket, srcKey)
	_, err := c.svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return fmt.Errorf("copy %s to %s: %w", srcKey, dstKey, err)
	}
	return nil
}

func encodeCopySource(bucket, key string) string {
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return url.PathEscape(bucket) + "/" + strings.Join(parts, "/")
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.svc.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for start := 0; start < len(keys); start += 1000 {
		end := start + 1000
		if end > len(keys) {
			end = len(keys)
		}
		ids := make([]types.ObjectIdentifier, 0, end-start)
		for _, key := range keys[start:end] {
			ids = append(ids, types.ObjectIdentifier{Key: aws.String(key)})
		}
		out, err := c.svc.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return fmt.Errorf("delete batch: %w", err)
		}
		if len(out.Errors) > 0 {
			return fmt.Errorf("delete failed for %d object(s), first: %s", len(out.Errors), aws.ToString(out.Errors[0].Key))
		}
	}
	return nil
}

func (c *Client) UploadFile(ctx context.Context, bucket, key, path string, progress func(done, total int64)) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	total := info.Size()
	listener := &progressListener{total: total, cb: progress}
	manager := transfermanager.New(c.svc, func(options *transfermanager.Options) {
		options.Concurrency = 4
		options.PartSizeBytes = 16 * 1024 * 1024
		options.MultipartUploadThreshold = 100 * 1024 * 1024
		if progress != nil {
			options.ObjectProgressListeners.Register(listener)
		}
	})
	input := &transfermanager.UploadObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          file,
		ContentLength: aws.Int64(total),
	}
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	_, err = manager.UploadObject(ctx, input)
	if err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	if progress != nil {
		progress(total, total)
	}
	return nil
}

type progressListener struct {
	total int64
	cb    func(int64, int64)
}

func (p *progressListener) OnObjectTransferStart(context.Context, *transfermanager.ObjectTransferStartEvent) {
	p.cb(0, p.total)
}

func (p *progressListener) OnObjectBytesTransferred(_ context.Context, event *transfermanager.ObjectBytesTransferredEvent) {
	p.cb(event.BytesTransferred, p.total)
}

func (c *Client) DownloadFile(ctx context.Context, bucket, key, destination string, progress func(done, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	out, err := c.svc.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	defer out.Body.Close()

	total := aws.ToInt64(out.ContentLength)
	file, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".r2go-part-*")
	if err != nil {
		return fmt.Errorf("create partial download for %s: %w", destination, err)
	}
	partial := file.Name()
	removePartial := true
	defer func() {
		_ = file.Close()
		if removePartial {
			_ = os.Remove(partial)
		}
	}()

	buf := make([]byte, 512*1024)
	var done int64
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, readErr := out.Body.Read(buf)
		if n > 0 {
			if _, err := file.Write(buf[:n]); err != nil {
				return fmt.Errorf("write %s: %w", destination, err)
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read %s: %w", key, readErr)
		}
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", destination, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", destination, err)
	}
	if err := os.Rename(partial, destination); err != nil {
		return fmt.Errorf("finish %s: %w", destination, err)
	}
	removePartial = false
	return nil
}
