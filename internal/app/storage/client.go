package storage

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// s3Client implements the StorageService interface, handling interactions with S3-compatible storage.
type s3Client struct {
	cfg      ServiceConfig
	s3Client *s3.Client
	uploader *manager.Uploader
}

// newS3Client initializes the S3 client using a custom configuration that supports S3-compatible endpoints.
func newS3Client(cfg ServiceConfig) (*s3Client, error) {
	// Load Configuration
	sdkCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKeyID,
			cfg.S3SecretAccessKey,
			"",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		log.Printf("Failed to load AWS SDK config: %v", err)
		return nil, errors.New("failed to initialize S3 client configuration")
	}

	// Create S3 Client with Custom Endpoint Resolver.
	client := s3.NewFromConfig(sdkCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = true
	})

	return &s3Client{
		cfg:      cfg,
		s3Client: client,
		uploader: manager.NewUploader(client),
	}, nil
}

// PresignUpload generates a presigned URL for uploading a file with the specified key, MIME type, and size.
func (c *s3Client) PresignUpload(
	ctx context.Context,
	key string,
	mimeType string,
	fileSize int64,
	duration time.Duration,
) (string, error) {
	presignClient := s3.NewPresignClient(c.s3Client)

	presignInput := &s3.PutObjectInput{
		Bucket:        &c.cfg.S3BucketName,
		Key:           &key,
		ContentType:   &mimeType,
		ContentLength: &fileSize,
	}

	resp, err := presignClient.PresignPutObject(
		ctx,
		presignInput,
		s3.WithPresignExpires(duration),
	)

	if err != nil {
		log.Printf("Failed to generate presigned upload URL for key %s: %v", key, err)
		return "", errors.New("failed to generate presigned upload URL")
	}

	return resp.URL, nil
}

// PresignDownload generates a presigned URL for downloading the specified file key.
func (c *s3Client) PresignDownload(ctx context.Context, key string, duration time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(c.s3Client)

	presignInput := &s3.GetObjectInput{
		Bucket: &c.cfg.S3BucketName,
		Key:    &key,
	}

	resp, err := presignClient.PresignGetObject(ctx, presignInput, s3.WithPresignExpires(duration))
	if err != nil {
		log.Printf("Failed to generate presigned URL for key %s: %v", key, err)
		return "", errors.New("failed to generate presigned URL")
	}

	return resp.URL, nil
}

// Delete removes the file specified by the given key from the bucket.
func (c *s3Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &c.cfg.S3BucketName,
		Key:    &key,
	})

	if err != nil {
		log.Printf("S3 delete failed for key %s: %v", key, err)
		return errors.New("failed to delete file from S3")
	}

	return nil
}

// GetObjectMetadata retrieves the metadata of an object.
func (c *s3Client) GetObjectMetadata(ctx context.Context, key string) (map[string]string, error) {
	resp, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &c.cfg.S3BucketName,
		Key:    &key,
	})

	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return nil, errors.New("file not found")
		}
		log.Printf("Failed to get S3 object metadata for key %s: %v", key, err)
		return nil, errors.New("failed to fetch S3 metadata")
	}

	metadata := make(map[string]string)
	if resp.ContentType != nil {
		metadata["Content-Type"] = *resp.ContentType
	}
	if resp.ContentLength != nil {
		metadata["Content-Length"] = strconv.FormatInt(*resp.ContentLength, 10)
	}

	return metadata, nil
}
