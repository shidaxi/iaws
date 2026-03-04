package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type ECRClient struct {
	client *ecr.Client
}

func NewECR(cfg aws.Config) *ECRClient {
	return &ECRClient{client: ecr.NewFromConfig(cfg)}
}

type ECRRepo struct {
	Name string
	URI  string
}

func (c *ECRClient) ListRepositories(ctx context.Context, nextToken *string, filter string) ([]ECRRepo, *string, error) {
	ilog.Info("ECR: ListRepositories (token=%v filter=%q)", nextToken != nil, filter)
	if filter != "" {
		var items []ECRRepo
		paginator := ecr.NewDescribeRepositoriesPaginator(c.client, &ecr.DescribeRepositoriesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, r := range page.Repositories {
				name := aws.ToString(r.RepositoryName)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					items = append(items, ECRRepo{Name: name, URI: aws.ToString(r.RepositoryUri)})
				}
			}
		}
		return items, nil, nil
	}
	input := &ecr.DescribeRepositoriesInput{MaxResults: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	out, err := c.client.DescribeRepositories(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []ECRRepo
	for _, r := range out.Repositories {
		items = append(items, ECRRepo{
			Name: aws.ToString(r.RepositoryName),
			URI:  aws.ToString(r.RepositoryUri),
		})
	}
	return items, out.NextToken, nil
}

func (c *ECRClient) ListImages(ctx context.Context, repoName string, nextToken *string, filter string) ([]string, *string, error) {
	ilog.Info("ECR: ListImages repo=%s (token=%v filter=%q)", repoName, nextToken != nil, filter)
	if filter != "" {
		var items []string
		paginator := ecr.NewDescribeImagesPaginator(c.client, &ecr.DescribeImagesInput{
			RepositoryName: aws.String(repoName),
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, img := range page.ImageDetails {
				line := formatECRImage(img)
				if strings.Contains(strings.ToLower(line), strings.ToLower(filter)) {
					items = append(items, line)
				}
			}
		}
		return items, nil, nil
	}
	input := &ecr.DescribeImagesInput{
		RepositoryName: aws.String(repoName),
		MaxResults:     aws.Int32(50),
	}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	out, err := c.client.DescribeImages(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []string
	for _, img := range out.ImageDetails {
		items = append(items, formatECRImage(img))
	}
	return items, out.NextToken, nil
}

func formatECRImage(img ecrtypes.ImageDetail) string {
	tags := "untagged"
	if len(img.ImageTags) > 0 {
		tags = strings.Join(img.ImageTags, ", ")
	}
	size := int64(0)
	if img.ImageSizeInBytes != nil {
		size = *img.ImageSizeInBytes / 1024 / 1024
	}
	pushed := ""
	if img.ImagePushedAt != nil {
		pushed = img.ImagePushedAt.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("%-30s %-8s %s", tags, fmt.Sprintf("%dMB", size), pushed)
}
