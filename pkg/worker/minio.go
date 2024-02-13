package worker

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

const (
	EnvMinioEndpoint  = "MINIO_ENDPOINT"
	EnvMinioAccessKey = "MINIO_ACCESS_KEY"
	EnvMinioSecretKey = "MINIO_SECRET_KEY"

	MinioFileLifetime = 24 * time.Hour

	bucketName = "testwave"
)

type Minio struct {
	client *minio.Client
}

type MinioConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Token     string
	Secure    bool
}

func NewMinio(conf MinioConfig) (*Minio, error) {
	cli, err := minio.New(conf.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(conf.AccessKey, conf.SecretKey, conf.Token),
		Secure: conf.Secure,
	})
	if err != nil {
		return nil, ErrInitMinioClient.Wrap(err)
	}

	return &Minio{client: cli}, nil
}

// PushToMinio pushes data (i.e. a reader) to Minio
func (m *Minio) Push(ctx context.Context, data io.Reader) (string, error) {
	if err := m.createBucketIfNotExists(ctx); err != nil {
		return "", err
	}

	uid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	contentID := uid.String()

	uploadInfo, err := m.client.PutObject(ctx, bucketName, contentID, data, -1,
		minio.PutObjectOptions{
			Expires: time.Now().Add(MinioFileLifetime),
		})
	if err != nil {
		return "", ErrPushingDataToMinio.Wrap(err)
	}

	logrus.Debugf("Data uploaded successfully to %s in bucket %s", uploadInfo.Key, bucketName)
	return contentID, nil
}

func (m *Minio) Pull(ctx context.Context, contentID string) (io.ReadCloser, error) {
	return m.client.GetObject(ctx, bucketName, contentID, minio.GetObjectOptions{})
}

// URL returns an S3-compatible URL for a Minio file
func (m *Minio) URL(ctx context.Context, contentID string) (string, error) {
	presignedURL, err := m.client.PresignedGetObject(ctx, bucketName, contentID, MinioFileLifetime, nil)
	if err != nil {
		return "", ErrGettingPresignedURL.Wrap(err)
	}

	return presignedURL.String(), nil
}

func (m *Minio) createBucketIfNotExists(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, bucketName)
	if err != nil {
		return ErrCheckingBucketExists.Wrap(err)
	}
	if exists {
		return nil
	}

	if err := m.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		return ErrCreatingBucket.Wrap(err)
	}
	logrus.Debugf("Bucket `%s` created successfully.", bucketName)

	return nil
}
