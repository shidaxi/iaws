package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type Route53Client struct {
	client *route53.Client
}

func NewRoute53(cfg aws.Config) *Route53Client {
	return &Route53Client{client: route53.NewFromConfig(cfg)}
}

type HostedZone struct {
	ID      string
	Name    string
	Private bool
	Records int64
}

func (c *Route53Client) ListHostedZones(ctx context.Context, marker *string, filter string) ([]HostedZone, *string, error) {
	ilog.Info("Route53: ListHostedZones (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []HostedZone
		var nextM *string
		for {
			input := &route53.ListHostedZonesInput{}
			if nextM != nil {
				input.Marker = nextM
			}
			out, err := c.client.ListHostedZones(ctx, input)
			if err != nil {
				ilog.Error("Route53: ListHostedZones search failed: %v", err)
				return nil, nil, err
			}
			for _, z := range out.HostedZones {
				name := aws.ToString(z.Name)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					id := aws.ToString(z.Id)
					id = strings.TrimPrefix(id, "/hostedzone/")
					items = append(items, HostedZone{
						ID:      id,
						Name:    name,
						Private: z.Config != nil && z.Config.PrivateZone,
						Records: aws.ToInt64(z.ResourceRecordSetCount),
					})
				}
			}
			if !out.IsTruncated {
				break
			}
			nextM = out.NextMarker
		}
		ilog.Info("Route53: search returned %d zones", len(items))
		return items, nil, nil
	}
	input := &route53.ListHostedZonesInput{MaxItems: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListHostedZones(ctx, input)
	if err != nil {
		ilog.Error("Route53: ListHostedZones failed: %v", err)
		return nil, nil, err
	}
	var items []HostedZone
	for _, z := range out.HostedZones {
		id := aws.ToString(z.Id)
		id = strings.TrimPrefix(id, "/hostedzone/")
		items = append(items, HostedZone{
			ID:      id,
			Name:    aws.ToString(z.Name),
			Private: z.Config != nil && z.Config.PrivateZone,
			Records: aws.ToInt64(z.ResourceRecordSetCount),
		})
	}
	var nextMarker *string
	if out.IsTruncated {
		nextMarker = out.NextMarker
	}
	ilog.Info("Route53: ListHostedZones returned %d zones", len(items))
	return items, nextMarker, nil
}

type RecordSet struct {
	Name   string
	Type   string
	TTL    int64
	Values []string
	Alias  string
}

func (c *Route53Client) ListRecordSets(ctx context.Context, zoneID string, startName *string, startType *string, filter string) ([]RecordSet, *string, *string, error) {
	ilog.Info("Route53: ListResourceRecordSets zone=%s filter=%q", zoneID, filter)
	if filter != "" {
		var items []RecordSet
		var nName *string
		var nType r53types.RRType
		hasNext := false
		for {
			inp := &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}
			if nName != nil {
				inp.StartRecordName = nName
			}
			if hasNext {
				inp.StartRecordType = nType
			}
			out, err := c.client.ListResourceRecordSets(ctx, inp)
			if err != nil {
				return nil, nil, nil, err
			}
			for _, r := range out.ResourceRecordSets {
				rs := buildRecordSet(r)
				search := strings.ToLower(rs.Name + " " + rs.Type)
				if strings.Contains(search, strings.ToLower(filter)) {
					items = append(items, rs)
				}
			}
			if !out.IsTruncated {
				break
			}
			nName = out.NextRecordName
			nType = out.NextRecordType
			hasNext = true
		}
		ilog.Info("Route53: search returned %d records", len(items))
		return items, nil, nil, nil
	}
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		MaxItems:     aws.Int32(50),
	}
	if startName != nil {
		input.StartRecordName = startName
	}
	if startType != nil {
		input.StartRecordType = r53types.RRType(*startType)
	}
	out, err := c.client.ListResourceRecordSets(ctx, input)
	if err != nil {
		ilog.Error("Route53: ListResourceRecordSets failed: %v", err)
		return nil, nil, nil, err
	}
	var items []RecordSet
	for _, r := range out.ResourceRecordSets {
		items = append(items, buildRecordSet(r))
	}
	var nextName, nextType *string
	if out.IsTruncated {
		nextName = out.NextRecordName
		nt := string(out.NextRecordType)
		nextType = &nt
	}
	ilog.Info("Route53: ListResourceRecordSets returned %d records", len(items))
	return items, nextName, nextType, nil
}

func buildRecordSet(r r53types.ResourceRecordSet) RecordSet {
	rs := RecordSet{
		Name: aws.ToString(r.Name),
		Type: string(r.Type),
	}
	if r.TTL != nil {
		rs.TTL = *r.TTL
	}
	if r.AliasTarget != nil {
		rs.Alias = aws.ToString(r.AliasTarget.DNSName)
	}
	for _, rr := range r.ResourceRecords {
		rs.Values = append(rs.Values, aws.ToString(rr.Value))
	}
	return rs
}

func FormatRecordSet(r RecordSet) string {
	vals := strings.Join(r.Values, ", ")
	if r.Alias != "" {
		vals = "ALIAS → " + r.Alias
	}
	return fmt.Sprintf("%-40s %-8s TTL:%-6d %s", r.Name, r.Type, r.TTL, vals)
}
