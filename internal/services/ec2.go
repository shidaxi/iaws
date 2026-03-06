package services

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type EC2Client struct {
	client *ec2.Client
}

func NewEC2(cfg aws.Config) *EC2Client {
	return &EC2Client{client: ec2.NewFromConfig(cfg)}
}

// InstanceItem holds data for list display.
type InstanceItem struct {
	ID       string
	State    string
	Type     string
	Name     string
	PublicIP string
}

func (c *EC2Client) ListInstances(ctx context.Context, nextToken *string, filter string) ([]InstanceItem, *string, error) {
	ilog.Info("EC2: ListInstances (token=%v filter=%q)", nextToken != nil, filter)
	input := &ec2.DescribeInstancesInput{MaxResults: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	if filter != "" {
		input.Filters = []types.Filter{
			{Name: aws.String("tag:Name"), Values: []string{"*" + filter + "*"}},
		}
	}
	out, err := c.client.DescribeInstances(ctx, input)
	if err != nil {
		ilog.Error("EC2: ListInstances failed: %v", err)
		return nil, nil, err
	}
	var items []InstanceItem
	for _, r := range out.Reservations {
		for _, i := range r.Instances {
			name := ""
			for _, t := range i.Tags {
				if aws.ToString(t.Key) == "Name" {
					name = aws.ToString(t.Value)
					break
				}
			}
			pubIP := ""
			if i.PublicIpAddress != nil {
				pubIP = *i.PublicIpAddress
			}
			items = append(items, InstanceItem{
				ID:       aws.ToString(i.InstanceId),
				State:    string(i.State.Name),
				Type:     string(i.InstanceType),
				Name:     name,
				PublicIP: pubIP,
			})
		}
	}
	ilog.Info("EC2: ListInstances returned %d items", len(items))
	return items, out.NextToken, nil
}

func (c *EC2Client) StartInstance(ctx context.Context, instanceID string) error {
	ilog.Info("EC2: StartInstance %s", instanceID)
	_, err := c.client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		ilog.Error("EC2: StartInstance %s failed: %v", instanceID, err)
	}
	return err
}

func (c *EC2Client) StopInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.StopInstances(ctx, &ec2.StopInstancesInput{InstanceIds: []string{instanceID}})
	return err
}

func (c *EC2Client) RebootInstance(ctx context.Context, instanceID string) error {
	_, err := c.client.RebootInstances(ctx, &ec2.RebootInstancesInput{InstanceIds: []string{instanceID}})
	return err
}

func (c *EC2Client) TerminateInstance(ctx context.Context, instanceID string) error {
	ilog.Info("EC2: TerminateInstance %s", instanceID)
	_, err := c.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{InstanceIds: []string{instanceID}})
	if err != nil {
		ilog.Error("EC2: TerminateInstance %s failed: %v", instanceID, err)
	}
	return err
}

// VPCItem holds data for list display.
type VPCItem struct {
	ID        string
	CIDR      string
	State     string
	IsDefault bool
}

func (c *EC2Client) ListVPCs(ctx context.Context) ([]VPCItem, error) {
	out, err := c.client.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, err
	}
	var items []VPCItem
	for _, v := range out.Vpcs {
		cidr := ""
		for _, assoc := range v.CidrBlockAssociationSet {
			if assoc.CidrBlock != nil && assoc.CidrBlockState != nil && assoc.CidrBlockState.State == types.VpcCidrBlockStateCodeAssociated {
				cidr = aws.ToString(assoc.CidrBlock)
				break
			}
		}
		items = append(items, VPCItem{
			ID:        aws.ToString(v.VpcId),
			CIDR:      cidr,
			State:     string(v.State),
			IsDefault: aws.ToBool(v.IsDefault),
		})
	}
	return items, nil
}

// SubnetItem holds data for list display.
type SubnetItem struct {
	ID     string
	VPCID  string
	CIDR   string
	AZ     string
	State  string
}

func (c *EC2Client) ListSubnets(ctx context.Context) ([]SubnetItem, error) {
	out, err := c.client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{})
	if err != nil {
		return nil, err
	}
	var items []SubnetItem
	for _, s := range out.Subnets {
		items = append(items, SubnetItem{
			ID:    aws.ToString(s.SubnetId),
			VPCID: aws.ToString(s.VpcId),
			CIDR:  aws.ToString(s.CidrBlock),
			AZ:    aws.ToString(s.AvailabilityZone),
			State: string(s.State),
		})
	}
	return items, nil
}

// SGItem holds data for list display.
type SGItem struct {
	ID   string
	Name string
	VPC  string
}

func (c *EC2Client) ListSecurityGroups(ctx context.Context) ([]SGItem, error) {
	out, err := c.client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		return nil, err
	}
	var items []SGItem
	for _, g := range out.SecurityGroups {
		items = append(items, SGItem{
			ID:   aws.ToString(g.GroupId),
			Name: aws.ToString(g.GroupName),
			VPC:  aws.ToString(g.VpcId),
		})
	}
	return items, nil
}

// KeyPairItem holds data for list display.
type KeyPairItem struct {
	Name   string
	KeyID  string
}

func (c *EC2Client) ListKeyPairs(ctx context.Context) ([]KeyPairItem, error) {
	out, err := c.client.DescribeKeyPairs(ctx, &ec2.DescribeKeyPairsInput{})
	if err != nil {
		return nil, err
	}
	var items []KeyPairItem
	for _, k := range out.KeyPairs {
		items = append(items, KeyPairItem{
			Name:  aws.ToString(k.KeyName),
			KeyID: aws.ToString(k.KeyPairId),
		})
	}
	return items, nil
}

// VolumeItem holds data for list display.
type VolumeItem struct {
	ID     string
	Size   int32
	State  string
	AZ     string
	Type   string
}

func (c *EC2Client) ListVolumes(ctx context.Context) ([]VolumeItem, error) {
	out, err := c.client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, err
	}
	var items []VolumeItem
	for _, v := range out.Volumes {
		size := int32(0)
		if v.Size != nil {
			size = *v.Size
		}
		items = append(items, VolumeItem{
			ID:    aws.ToString(v.VolumeId),
			Size:  size,
			State: string(v.State),
			AZ:    aws.ToString(v.AvailabilityZone),
			Type:  string(v.VolumeType),
		})
	}
	return items, nil
}

// SnapshotItem holds data for list display.
type SnapshotItem struct {
	ID          string
	VolumeID    string
	Size        int32
	State       string
	Description string
	StartTime   string
}

func (c *EC2Client) ListSnapshots(ctx context.Context) ([]SnapshotItem, error) {
	ilog.Info("EC2: ListSnapshots (owner=self)")
	out, err := c.client.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
		OwnerIds: []string{"self"},
	})
	if err != nil {
		ilog.Error("EC2: ListSnapshots failed: %v", err)
		return nil, err
	}
	var items []SnapshotItem
	for _, s := range out.Snapshots {
		size := int32(0)
		if s.VolumeSize != nil {
			size = *s.VolumeSize
		}
		startTime := ""
		if s.StartTime != nil {
			startTime = s.StartTime.Format("2006-01-02 15:04")
		}
		desc := aws.ToString(s.Description)
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		items = append(items, SnapshotItem{
			ID:          aws.ToString(s.SnapshotId),
			VolumeID:    aws.ToString(s.VolumeId),
			Size:        size,
			State:       string(s.State),
			Description: desc,
			StartTime:   startTime,
		})
	}
	ilog.Info("EC2: ListSnapshots returned %d items", len(items))
	return items, nil
}

// AMIItem holds data for list display.
type AMIItem struct {
	ID   string
	Name string
	State string
}

func (c *EC2Client) ListAMIs(ctx context.Context, self bool) ([]AMIItem, error) {
	input := &ec2.DescribeImagesInput{}
	if self {
		input.Owners = []string{"self"}
	}
	out, err := c.client.DescribeImages(ctx, input)
	if err != nil {
		return nil, err
	}
	var items []AMIItem
	for _, i := range out.Images {
		name := aws.ToString(i.ImageId)
		for _, t := range i.Tags {
			if aws.ToString(t.Key) == "Name" {
				name = aws.ToString(t.Value)
				break
			}
		}
		items = append(items, AMIItem{
			ID:    aws.ToString(i.ImageId),
			Name:  name,
			State: string(i.State),
		})
	}
	return items, nil
}

// InstanceIDForSSM returns the instance ID; SSM Session Manager requires the instance to be running with SSM Agent installed.
func (c *EC2Client) InstanceIDForSSM(ctx context.Context, instanceID string) (string, error) {
	return instanceID, nil
}

