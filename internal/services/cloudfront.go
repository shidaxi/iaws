package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type CloudFrontClient struct {
	client *cloudfront.Client
}

func NewCloudFront(cfg aws.Config) *CloudFrontClient {
	return &CloudFrontClient{client: cloudfront.NewFromConfig(cfg)}
}

type DistItem struct {
	ID      string
	Domain  string
	Status  string
	Aliases string
	Comment string
	Enabled bool
}

func (c *CloudFrontClient) ListDistributions(ctx context.Context, marker *string, filter string) ([]DistItem, *string, error) {
	ilog.Info("CloudFront: ListDistributions (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []DistItem
		var nm *string
		for {
			input := &cloudfront.ListDistributionsInput{}
			if nm != nil {
				input.Marker = nm
			}
			out, err := c.client.ListDistributions(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			if out.DistributionList != nil {
				for _, d := range out.DistributionList.Items {
					item := buildDistItem(d)
					search := strings.ToLower(item.Domain + " " + item.Aliases + " " + item.Comment)
					if strings.Contains(search, strings.ToLower(filter)) {
						items = append(items, item)
					}
				}
				if !aws.ToBool(out.DistributionList.IsTruncated) {
					break
				}
				nm = out.DistributionList.NextMarker
			} else {
				break
			}
		}
		return items, nil, nil
	}
	input := &cloudfront.ListDistributionsInput{MaxItems: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListDistributions(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []DistItem
	var next *string
	if out.DistributionList != nil {
		for _, d := range out.DistributionList.Items {
			items = append(items, buildDistItem(d))
		}
		if aws.ToBool(out.DistributionList.IsTruncated) {
			next = out.DistributionList.NextMarker
		}
	}
	return items, next, nil
}

func buildDistItem(d cftypes.DistributionSummary) DistItem {
	aliases := ""
	if d.Aliases != nil && len(d.Aliases.Items) > 0 {
		aliases = strings.Join(d.Aliases.Items, ", ")
	}
	return DistItem{
		ID:      aws.ToString(d.Id),
		Domain:  aws.ToString(d.DomainName),
		Status:  aws.ToString(d.Status),
		Aliases: aliases,
		Comment: aws.ToString(d.Comment),
		Enabled: aws.ToBool(d.Enabled),
	}
}

func FormatDistDetail(d DistItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:        %s\n", d.ID)
	fmt.Fprintf(&b, "Domain:    %s\n", d.Domain)
	fmt.Fprintf(&b, "Status:    %s\n", d.Status)
	fmt.Fprintf(&b, "Enabled:   %v\n", d.Enabled)
	if d.Aliases != "" {
		fmt.Fprintf(&b, "Aliases:   %s\n", d.Aliases)
	}
	if d.Comment != "" {
		fmt.Fprintf(&b, "Comment:   %s\n", d.Comment)
	}
	return b.String()
}
