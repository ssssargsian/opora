package document

import (
	"context"
	"io"
	"net/url"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"opora.local/api/internal/config"
)

type Storage interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

type S3Storage struct {
	client *minio.Client
	bucket string
}

func NewS3Storage(cfg config.Storage) (*S3Storage, error) {
	endpoint := cfg.Endpoint
	if parsed, parseErr := url.Parse(cfg.Endpoint); parseErr == nil && parsed.Host != "" {
		endpoint = parsed.Host
	}
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.UseTLS, Region: cfg.Region})
	if err != nil {
		return nil, err
	}
	return &S3Storage{client: client, bucket: cfg.Bucket}, nil
}
func (s *S3Storage) Put(ctx context.Context, key string, reader io.Reader, size int64, mime string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: mime, DisableContentSha256: false})
	return err
}
func (s *S3Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
