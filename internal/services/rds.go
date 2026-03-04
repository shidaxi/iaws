package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type RDSClient struct {
	client *rds.Client
}

func NewRDS(cfg aws.Config) *RDSClient {
	return &RDSClient{client: rds.NewFromConfig(cfg)}
}

type DBInstance struct {
	ID       string
	Engine   string
	Class    string
	Status   string
	Endpoint string
	Port     int32
	MultiAZ  bool
	Storage  int32
}

func (c *RDSClient) ListDBInstances(ctx context.Context, marker *string, filter string) ([]DBInstance, *string, error) {
	ilog.Info("RDS: ListDBInstances (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []DBInstance
		paginator := rds.NewDescribeDBInstancesPaginator(c.client, &rds.DescribeDBInstancesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, db := range page.DBInstances {
				item := buildDBInstance(db)
				if strings.Contains(strings.ToLower(item.ID+" "+item.Engine), strings.ToLower(filter)) {
					items = append(items, item)
				}
			}
		}
		return items, nil, nil
	}
	input := &rds.DescribeDBInstancesInput{MaxRecords: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.DescribeDBInstances(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []DBInstance
	for _, db := range out.DBInstances {
		items = append(items, buildDBInstance(db))
	}
	return items, out.Marker, nil
}

func buildDBInstance(db rdstypes.DBInstance) DBInstance {
	ep := ""
	port := int32(0)
	if db.Endpoint != nil {
		ep = aws.ToString(db.Endpoint.Address)
		if db.Endpoint.Port != nil {
			port = *db.Endpoint.Port
		}
	}
	storage := int32(0)
	if db.AllocatedStorage != nil {
		storage = *db.AllocatedStorage
	}
	return DBInstance{
		ID:       aws.ToString(db.DBInstanceIdentifier),
		Engine:   aws.ToString(db.Engine) + " " + aws.ToString(db.EngineVersion),
		Class:    aws.ToString(db.DBInstanceClass),
		Status:   aws.ToString(db.DBInstanceStatus),
		Endpoint: ep,
		Port:     port,
		MultiAZ:  aws.ToBool(db.MultiAZ),
		Storage:  storage,
	}
}

func FormatDBDetail(db DBInstance) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:        %s\n", db.ID)
	fmt.Fprintf(&b, "Engine:    %s\n", db.Engine)
	fmt.Fprintf(&b, "Class:     %s\n", db.Class)
	fmt.Fprintf(&b, "Status:    %s\n", db.Status)
	fmt.Fprintf(&b, "Storage:   %d GiB\n", db.Storage)
	fmt.Fprintf(&b, "MultiAZ:   %v\n", db.MultiAZ)
	if db.Endpoint != "" {
		fmt.Fprintf(&b, "Endpoint:  %s:%d\n", db.Endpoint, db.Port)
	}
	return b.String()
}
