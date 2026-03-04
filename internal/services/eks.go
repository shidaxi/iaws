package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type EKSClient struct {
	client *eks.Client
}

func NewEKS(cfg aws.Config) *EKSClient {
	return &EKSClient{client: eks.NewFromConfig(cfg)}
}

func (c *EKSClient) ListClusters(ctx context.Context, nextToken *string, filter string) ([]string, *string, error) {
	ilog.Info("EKS: ListClusters (token=%v filter=%q)", nextToken != nil, filter)
	if filter != "" {
		var items []string
		var nt *string
		for {
			input := &eks.ListClustersInput{}
			if nt != nil {
				input.NextToken = nt
			}
			out, err := c.client.ListClusters(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			for _, name := range out.Clusters {
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					items = append(items, name)
				}
			}
			if out.NextToken == nil {
				break
			}
			nt = out.NextToken
		}
		return items, nil, nil
	}
	input := &eks.ListClustersInput{MaxResults: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	out, err := c.client.ListClusters(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	return out.Clusters, out.NextToken, nil
}

func (c *EKSClient) DescribeCluster(ctx context.Context, name string) (string, error) {
	ilog.Info("EKS: DescribeCluster %s", name)
	out, err := c.client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
	if err != nil {
		return "", err
	}
	cl := out.Cluster
	var b strings.Builder
	fmt.Fprintf(&b, "Name:            %s\n", aws.ToString(cl.Name))
	fmt.Fprintf(&b, "Status:          %s\n", cl.Status)
	fmt.Fprintf(&b, "Version:         %s\n", aws.ToString(cl.Version))
	fmt.Fprintf(&b, "Platform:        %s\n", aws.ToString(cl.PlatformVersion))
	fmt.Fprintf(&b, "Endpoint:        %s\n", aws.ToString(cl.Endpoint))
	fmt.Fprintf(&b, "Role ARN:        %s\n", aws.ToString(cl.RoleArn))
	fmt.Fprintf(&b, "ARN:             %s\n", aws.ToString(cl.Arn))
	if cl.ResourcesVpcConfig != nil {
		fmt.Fprintf(&b, "VPC ID:          %s\n", aws.ToString(cl.ResourcesVpcConfig.VpcId))
		fmt.Fprintf(&b, "Subnets:         %s\n", strings.Join(cl.ResourcesVpcConfig.SubnetIds, ", "))
	}
	if cl.KubernetesNetworkConfig != nil {
		fmt.Fprintf(&b, "Service CIDR:    %s\n", aws.ToString(cl.KubernetesNetworkConfig.ServiceIpv4Cidr))
	}
	return b.String(), nil
}
