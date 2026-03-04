package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type ELBClient struct {
	client *elbv2.Client
}

func NewELB(cfg aws.Config) *ELBClient {
	return &ELBClient{client: elbv2.NewFromConfig(cfg)}
}

type LBItem struct {
	Name   string
	DNS    string
	Type   string
	Scheme string
	State  string
	VPC    string
	ARN    string
}

func (c *ELBClient) ListLoadBalancers(ctx context.Context, marker *string, filter string) ([]LBItem, *string, error) {
	ilog.Info("ELB: ListLoadBalancers (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []LBItem
		paginator := elbv2.NewDescribeLoadBalancersPaginator(c.client, &elbv2.DescribeLoadBalancersInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, lb := range page.LoadBalancers {
				item := buildLBItem(lb)
				if strings.Contains(strings.ToLower(item.Name+" "+item.DNS), strings.ToLower(filter)) {
					items = append(items, item)
				}
			}
		}
		return items, nil, nil
	}
	input := &elbv2.DescribeLoadBalancersInput{PageSize: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.DescribeLoadBalancers(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []LBItem
	for _, lb := range out.LoadBalancers {
		items = append(items, buildLBItem(lb))
	}
	return items, out.NextMarker, nil
}

func buildLBItem(lb elbv2types.LoadBalancer) LBItem {
	state := ""
	if lb.State != nil {
		state = string(lb.State.Code)
	}
	return LBItem{
		Name:   aws.ToString(lb.LoadBalancerName),
		DNS:    aws.ToString(lb.DNSName),
		Type:   string(lb.Type),
		Scheme: string(lb.Scheme),
		State:  state,
		VPC:    aws.ToString(lb.VpcId),
		ARN:    aws.ToString(lb.LoadBalancerArn),
	}
}

func FormatLBDetail(lb LBItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:     %s\n", lb.Name)
	fmt.Fprintf(&b, "Type:     %s\n", lb.Type)
	fmt.Fprintf(&b, "Scheme:   %s\n", lb.Scheme)
	fmt.Fprintf(&b, "State:    %s\n", lb.State)
	fmt.Fprintf(&b, "DNS:      %s\n", lb.DNS)
	fmt.Fprintf(&b, "VPC:      %s\n", lb.VPC)
	fmt.Fprintf(&b, "ARN:      %s\n", lb.ARN)
	return b.String()
}
