package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type KMSClient struct {
	client *kms.Client
}

func NewKMS(cfg aws.Config) *KMSClient {
	return &KMSClient{client: kms.NewFromConfig(cfg)}
}

type KMSKeyItem struct {
	Alias string
	ID    string
}

func (c *KMSClient) fetchAliasMap(ctx context.Context) map[string]string {
	m := make(map[string]string)
	var marker *string
	for {
		input := &kms.ListAliasesInput{}
		if marker != nil {
			input.Marker = marker
		}
		out, err := c.client.ListAliases(ctx, input)
		if err != nil {
			ilog.Error("KMS: ListAliases failed: %v", err)
			return m
		}
		for _, a := range out.Aliases {
			if a.TargetKeyId != nil {
				m[aws.ToString(a.TargetKeyId)] = aws.ToString(a.AliasName)
			}
		}
		if !out.Truncated {
			break
		}
		marker = out.NextMarker
	}
	return m
}

func (c *KMSClient) ListKeys(ctx context.Context, marker *string, filter string) ([]KMSKeyItem, *string, error) {
	ilog.Info("KMS: ListKeys (marker=%v filter=%q)", marker != nil, filter)
	aliasMap := c.fetchAliasMap(ctx)
	if filter != "" {
		var items []KMSKeyItem
		var nt *string
		for {
			input := &kms.ListKeysInput{}
			if nt != nil {
				input.Marker = nt
			}
			out, err := c.client.ListKeys(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			for _, k := range out.Keys {
				id := aws.ToString(k.KeyId)
				alias := aliasMap[id]
				search := strings.ToLower(id + " " + alias)
				if strings.Contains(search, strings.ToLower(filter)) {
					items = append(items, KMSKeyItem{ID: id, Alias: alias})
				}
			}
			if !out.Truncated {
				break
			}
			nt = out.NextMarker
		}
		return items, nil, nil
	}
	input := &kms.ListKeysInput{Limit: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListKeys(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []KMSKeyItem
	for _, k := range out.Keys {
		id := aws.ToString(k.KeyId)
		items = append(items, KMSKeyItem{ID: id, Alias: aliasMap[id]})
	}
	var next *string
	if out.Truncated {
		next = out.NextMarker
	}
	return items, next, nil
}

func (c *KMSClient) DescribeKey(ctx context.Context, keyID string) (string, error) {
	ilog.Info("KMS: DescribeKey %s", keyID)
	out, err := c.client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return "", err
	}
	k := out.KeyMetadata

	aliasMap := c.fetchAliasMap(ctx)
	alias := aliasMap[aws.ToString(k.KeyId)]

	var b strings.Builder
	if alias != "" {
		fmt.Fprintf(&b, "Alias:        %s\n", alias)
	}
	fmt.Fprintf(&b, "Key ID:       %s\n", aws.ToString(k.KeyId))
	fmt.Fprintf(&b, "ARN:          %s\n", aws.ToString(k.Arn))
	fmt.Fprintf(&b, "Description:  %s\n", aws.ToString(k.Description))
	fmt.Fprintf(&b, "State:        %s\n", k.KeyState)
	fmt.Fprintf(&b, "Key Usage:    %s\n", k.KeyUsage)
	fmt.Fprintf(&b, "Key Spec:     %s\n", k.KeySpec)
	fmt.Fprintf(&b, "Origin:       %s\n", k.Origin)
	fmt.Fprintf(&b, "Manager:      %s\n", k.KeyManager)
	if k.CreationDate != nil {
		fmt.Fprintf(&b, "Created:      %s\n", k.CreationDate.Format("2006-01-02 15:04:05"))
	}
	return b.String(), nil
}
