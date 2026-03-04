package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type IAMClient struct {
	client *iam.Client
}

func NewIAM(cfg aws.Config) *IAMClient {
	return &IAMClient{client: iam.NewFromConfig(cfg)}
}

type IAMUser struct {
	Name    string
	ARN     string
	Created string
}

func (c *IAMClient) ListUsers(ctx context.Context, marker *string, filter string) ([]IAMUser, *string, error) {
	ilog.Info("IAM: ListUsers (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []IAMUser
		paginator := iam.NewListUsersPaginator(c.client, &iam.ListUsersInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, u := range page.Users {
				name := aws.ToString(u.UserName)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					created := ""
					if u.CreateDate != nil {
						created = u.CreateDate.Format("2006-01-02")
					}
					items = append(items, IAMUser{Name: name, ARN: aws.ToString(u.Arn), Created: created})
				}
			}
		}
		return items, nil, nil
	}
	input := &iam.ListUsersInput{MaxItems: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListUsers(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []IAMUser
	for _, u := range out.Users {
		created := ""
		if u.CreateDate != nil {
			created = u.CreateDate.Format("2006-01-02")
		}
		items = append(items, IAMUser{
			Name:    aws.ToString(u.UserName),
			ARN:     aws.ToString(u.Arn),
			Created: created,
		})
	}
	var next *string
	if out.IsTruncated {
		next = out.Marker
	}
	return items, next, nil
}

type IAMRole struct {
	Name    string
	ARN     string
	Created string
	Desc    string
}

func (c *IAMClient) ListRoles(ctx context.Context, marker *string, filter string) ([]IAMRole, *string, error) {
	ilog.Info("IAM: ListRoles (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []IAMRole
		paginator := iam.NewListRolesPaginator(c.client, &iam.ListRolesInput{})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, r := range page.Roles {
				name := aws.ToString(r.RoleName)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					created := ""
					if r.CreateDate != nil {
						created = r.CreateDate.Format("2006-01-02")
					}
					items = append(items, IAMRole{
						Name: name, ARN: aws.ToString(r.Arn),
						Created: created, Desc: aws.ToString(r.Description),
					})
				}
			}
		}
		return items, nil, nil
	}
	input := &iam.ListRolesInput{MaxItems: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListRoles(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []IAMRole
	for _, r := range out.Roles {
		created := ""
		if r.CreateDate != nil {
			created = r.CreateDate.Format("2006-01-02")
		}
		items = append(items, IAMRole{
			Name:    aws.ToString(r.RoleName),
			ARN:     aws.ToString(r.Arn),
			Created: created,
			Desc:    aws.ToString(r.Description),
		})
	}
	var next *string
	if out.IsTruncated {
		next = out.Marker
	}
	return items, next, nil
}

type IAMPolicy struct {
	Name            string
	ARN             string
	AttachmentCount int32
	Desc            string
}

func (c *IAMClient) ListPolicies(ctx context.Context, marker *string, filter string) ([]IAMPolicy, *string, error) {
	ilog.Info("IAM: ListPolicies (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []IAMPolicy
		paginator := iam.NewListPoliciesPaginator(c.client, &iam.ListPoliciesInput{Scope: "Local"})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, nil, err
			}
			for _, p := range page.Policies {
				name := aws.ToString(p.PolicyName)
				if strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
					cnt := int32(0)
					if p.AttachmentCount != nil {
						cnt = *p.AttachmentCount
					}
					items = append(items, IAMPolicy{
						Name: name, ARN: aws.ToString(p.Arn),
						AttachmentCount: cnt, Desc: aws.ToString(p.Description),
					})
				}
			}
		}
		return items, nil, nil
	}
	input := &iam.ListPoliciesInput{MaxItems: aws.Int32(50), Scope: "Local"}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListPolicies(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []IAMPolicy
	for _, p := range out.Policies {
		cnt := int32(0)
		if p.AttachmentCount != nil {
			cnt = *p.AttachmentCount
		}
		items = append(items, IAMPolicy{
			Name:            aws.ToString(p.PolicyName),
			ARN:             aws.ToString(p.Arn),
			AttachmentCount: cnt,
			Desc:            aws.ToString(p.Description),
		})
	}
	var next *string
	if out.IsTruncated {
		next = out.Marker
	}
	return items, next, nil
}

func FormatUserDetail(u IAMUser) string {
	return fmt.Sprintf("Name:     %s\nARN:      %s\nCreated:  %s", u.Name, u.ARN, u.Created)
}

func FormatRoleDetail(r IAMRole) string {
	return fmt.Sprintf("Name:     %s\nARN:      %s\nCreated:  %s\nDesc:     %s", r.Name, r.ARN, r.Created, r.Desc)
}

func FormatPolicyDetail(p IAMPolicy) string {
	return fmt.Sprintf("Name:       %s\nARN:        %s\nAttached:   %d\nDesc:       %s", p.Name, p.ARN, p.AttachmentCount, p.Desc)
}
