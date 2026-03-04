package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type S3Client struct {
	client *s3.Client
}

func NewS3(cfg aws.Config) *S3Client {
	return &S3Client{client: s3.NewFromConfig(cfg)}
}

func (c *S3Client) ListBuckets(ctx context.Context) ([]string, error) {
	ilog.Info("S3: ListBuckets")
	out, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		ilog.Error("S3: ListBuckets failed: %v", err)
		return nil, err
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		names = append(names, aws.ToString(b.Name))
	}
	return names, nil
}

// ObjectItem for list display (prefix or key).
type ObjectItem struct {
	Key  string
	Size int64
	IsDir bool
}

func (c *S3Client) ListObjects(ctx context.Context, bucket, prefix string, contToken *string, filter string) ([]ObjectItem, *string, error) {
	ilog.Info("S3: ListObjects bucket=%s prefix=%s (token=%v filter=%q)", bucket, prefix, contToken != nil, filter)
	if filter != "" {
		var items []ObjectItem
		paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
			Bucket:    aws.String(bucket),
			Prefix:    aws.String(prefix),
			Delimiter: aws.String("/"),
		})
		fl := strings.ToLower(filter)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				ilog.Error("S3: ListObjects search failed: %v", err)
				return nil, nil, err
			}
			for _, cp := range page.CommonPrefixes {
				key := aws.ToString(cp.Prefix)
				if strings.Contains(strings.ToLower(key), fl) {
					items = append(items, ObjectItem{Key: key, IsDir: true})
				}
			}
			for _, o := range page.Contents {
				key := aws.ToString(o.Key)
				if strings.Contains(strings.ToLower(key), fl) {
					items = append(items, ObjectItem{Key: key, Size: aws.ToInt64(o.Size), IsDir: false})
				}
			}
		}
		ilog.Info("S3: search returned %d items", len(items))
		return items, nil, nil
	}
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		MaxKeys:   aws.Int32(50),
		Delimiter: aws.String("/"),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	if contToken != nil {
		input.ContinuationToken = contToken
	}
	out, err := c.client.ListObjectsV2(ctx, input)
	if err != nil {
		ilog.Error("S3: ListObjects failed: %v", err)
		return nil, nil, err
	}
	var items []ObjectItem
	for _, cp := range out.CommonPrefixes {
		items = append(items, ObjectItem{Key: aws.ToString(cp.Prefix), IsDir: true})
	}
	for _, o := range out.Contents {
		items = append(items, ObjectItem{
			Key:   aws.ToString(o.Key),
			Size:  aws.ToInt64(o.Size),
			IsDir: false,
		})
	}
	var nextToken *string
	if aws.ToBool(out.IsTruncated) {
		nextToken = out.NextContinuationToken
	}
	ilog.Info("S3: ListObjects returned %d items", len(items))
	return items, nextToken, nil
}

func (c *S3Client) GetObject(ctx context.Context, bucket, key, localPath string) error {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer out.Body.Close()
	if localPath == "" {
		localPath = filepath.Base(key)
	}
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, out.Body)
	return err
}

func (c *S3Client) PutObject(ctx context.Context, bucket, key, localPath string) error {
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	return err
}
