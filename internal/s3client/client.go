package s3client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"r2go/internal/config"
)

type Client struct {
	Account config.Account
	svc     *s3.Client
	presign *s3.PresignClient
}

func New(acc config.Account, secret string) *Client {
	base := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", acc.AccountID)
	cfg := aws.Config{
		Region: "auto",
		Credentials: credentials.NewStaticCredentialsProvider(
			acc.AccessKeyID, secret, "",
		),
	}
	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(base)
		o.UsePathStyle = true
	})
	return &Client{
		Account: acc,
		svc:     svc,
		presign: s3.NewPresignClient(svc),
	}
}

func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	out, err := c.svc.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %w", err)
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		if b.Name != nil {
			names = append(names, *b.Name)
		}
	}
	return names, nil
}

type Object struct {
	Key     string
	Size    int64
	ModTime time.Time
	ETag    string
}

type Prefix struct {
	Prefix string
}

type ListPage struct {
	Objects   []Object
	Prefixes  []Prefix
	NextToken string
	Truncated bool
}

func (c *Client) ListPage(ctx context.Context, bucket, prefix, token string) (ListPage, error) {
	in := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	}
	if token != "" {
		in.ContinuationToken = aws.String(token)
	}
	out, err := c.svc.ListObjectsV2(ctx, in)
	if err != nil {
		return ListPage{}, fmt.Errorf("list objects: %w", err)
	}
	page := ListPage{Truncated: out.IsTruncated != nil && *out.IsTruncated}
	if out.NextContinuationToken != nil {
		page.NextToken = *out.NextContinuationToken
	}
	for _, o := range out.Contents {
		if o.Key == nil || *o.Key == prefix {
			continue
		}
		mod := time.Time{}
		if o.LastModified != nil {
			mod = *o.LastModified
		}
		page.Objects = append(page.Objects, Object{
			Key:     deref(o.Key),
			Size:    deref64(o.Size),
			ModTime: mod,
			ETag:    deref(o.ETag),
		})
	}
	for _, p := range out.CommonPrefixes {
		if p.Prefix != nil {
			page.Prefixes = append(page.Prefixes, Prefix{Prefix: *p.Prefix})
		}
	}
	return page, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func deref64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

func (c *Client) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, progress func(int64)) error {
	opts := []func(*transfermanager.Options){
		func(o *transfermanager.Options) {
			o.Concurrency = 4
			o.PartSizeBytes = 16 * 1024 * 1024
		},
	}
	if progress != nil {
		opts = append(opts, func(o *transfermanager.Options) {
			o.ObjectProgressListeners.Register(&progressListener{cb: progress})
		})
	}
	tm := transfermanager.New(c.svc, opts...)
	_, err := tm.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

type progressListener struct {
	cb func(int64)
}

func (l *progressListener) OnObjectBytesTransferred(_ context.Context, e *transfermanager.ObjectBytesTransferredEvent) {
	l.cb(e.BytesTransferred)
}

func (l *progressListener) OnObjectTransferStart(_ context.Context, e *transfermanager.ObjectTransferStartEvent) {
	l.cb(0)
}

func (c *Client) Download(ctx context.Context, bucket, key string, dest io.Writer, offset int64, progress func(int64)) error {
	in := &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)}
	if offset > 0 {
		in.Range = aws.String(fmt.Sprintf("bytes=%d-", offset))
	}
	out, err := c.svc.GetObject(ctx, in)
	if err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	defer out.Body.Close()

	buf := make([]byte, 256*1024)
	total := offset
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		n, rerr := out.Body.Read(buf)
		if n > 0 {
			if _, werr := dest.Write(buf[:n]); werr != nil {
				return fmt.Errorf("download %s: %w", key, werr)
			}
			total += int64(n)
			if progress != nil {
				progress(total)
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return fmt.Errorf("download %s: %w", key, rerr)
		}
	}
}

func (c *Client) ListAll(ctx context.Context, bucket, prefix string) ([]Object, error) {
	var all []Object
	var token *string
	for {
		out, err := c.svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list all: %w", err)
		}
		for _, o := range out.Contents {
			if o.Key == nil {
				continue
			}
			mod := time.Time{}
			if o.LastModified != nil {
				mod = *o.LastModified
			}
			all = append(all, Object{Key: *o.Key, Size: deref64(o.Size), ModTime: mod, ETag: deref(o.ETag)})
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}
	return all, nil
}

func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string, meta map[string]string) error {
	in := &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if len(meta) > 0 {
		in.Metadata = meta
	}
	if _, err := c.svc.PutObject(ctx, in); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	if _, err := c.svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}

func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objs := make([]types.ObjectIdentifier, 0, len(keys))
	for _, k := range keys {
		objs = append(objs, types.ObjectIdentifier{Key: aws.String(k)})
	}
	out, err := c.svc.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return fmt.Errorf("delete batch: %w", err)
	}
	if len(out.Errors) > 0 {
		return fmt.Errorf("delete failed for %d object(s), e.g. %s: %s",
			len(out.Errors), deref(out.Errors[0].Key), deref(out.Errors[0].Message))
	}
	return nil
}

func (c *Client) CopyObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	_, err := c.svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(dstKey),
		CopySource: aws.String(bucket + "/" + srcKey),
	})
	if err != nil {
		return fmt.Errorf("copy %s -> %s: %w", srcKey, dstKey, err)
	}
	return nil
}

func (c *Client) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	out, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign %s: %w", key, err)
	}
	return out.URL, nil
}

func (c *Client) UpdateMetadata(ctx context.Context, bucket, key, contentType, cacheControl string, meta map[string]string) error {
	_, err := c.svc.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:            aws.String(bucket),
		Key:               aws.String(key),
		CopySource:        aws.String(bucket + "/" + key),
		MetadataDirective: types.MetadataDirectiveReplace,
		ContentType:       aws.String(contentType),
		CacheControl:      aws.String(cacheControl),
		Metadata:          meta,
	})
	if err != nil {
		return fmt.Errorf("update metadata %s: %w", key, err)
	}
	return nil
}

func (c *Client) GetRange(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, error) {
	out, err := c.svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, fmt.Errorf("range %s: %w", key, err)
	}
	return out.Body, nil
}

func (c *Client) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	out, err := c.svc.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("head %s: %w", key, err)
	}
	return out, nil
}

func IsNotFound(err error) bool {
	var nf *types.NoSuchKey
	return errors.As(err, &nf)
}
