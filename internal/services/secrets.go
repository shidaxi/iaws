package services

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type SecretsClient struct {
	client *secretsmanager.Client
}

func NewSecrets(cfg aws.Config) *SecretsClient {
	return &SecretsClient{client: secretsmanager.NewFromConfig(cfg)}
}

// SecretItem holds data for list display.
type SecretItem struct {
	Name string
	ARN  string
}

func (c *SecretsClient) ListSecrets(ctx context.Context, nextToken *string, filter string) ([]SecretItem, *string, error) {
	ilog.Info("Secrets: ListSecrets (token=%v filter=%q)", nextToken != nil, filter)
	if filter != "" {
		var items []SecretItem
		paginator := secretsmanager.NewListSecretsPaginator(c.client, &secretsmanager.ListSecretsInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				ilog.Error("Secrets: ListSecrets search failed: %v", err)
				return nil, nil, err
			}
			for _, s := range page.SecretList {
				name := aws.ToString(s.Name)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					items = append(items, SecretItem{Name: name, ARN: aws.ToString(s.ARN)})
				}
			}
		}
		ilog.Info("Secrets: search returned %d items", len(items))
		return items, nil, nil
	}
	input := &secretsmanager.ListSecretsInput{MaxResults: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	out, err := c.client.ListSecrets(ctx, input)
	if err != nil {
		ilog.Error("Secrets: ListSecrets failed: %v", err)
		return nil, nil, err
	}
	var items []SecretItem
	for _, s := range out.SecretList {
		items = append(items, SecretItem{
			Name: aws.ToString(s.Name),
			ARN:  aws.ToString(s.ARN),
		})
	}
	ilog.Info("Secrets: ListSecrets returned %d items", len(items))
	return items, out.NextToken, nil
}

func (c *SecretsClient) GetSecretValue(ctx context.Context, nameOrID string) (string, error) {
	out, err := c.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(nameOrID),
	})
	if err != nil {
		return "", err
	}
	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if out.SecretBinary != nil {
		return string(out.SecretBinary), nil
	}
	return "", nil
}

func (c *SecretsClient) PutSecretValue(ctx context.Context, name, value string) error {
	_, err := c.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretString: aws.String(value),
	})
	return err
}

func (c *SecretsClient) CreateSecret(ctx context.Context, name, value string) error {
	_, err := c.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretString: aws.String(value),
	})
	return err
}
