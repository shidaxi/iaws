package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type SSMClient struct {
	client  *ssm.Client
	region  string
	profile string
}

func NewSSM(cfg aws.Config, profile string) *SSMClient {
	return &SSMClient{
		client:  ssm.NewFromConfig(cfg),
		region:  cfg.Region,
		profile: profile,
	}
}

type ParameterItem struct {
	Name  string
	Type  string
	Value string
}

func (c *SSMClient) ListParameters(ctx context.Context, nextToken *string, filter string) ([]ParameterItem, *string, error) {
	ilog.Info("SSM: ListParameters (token=%v filter=%q)", nextToken != nil, filter)
	input := &ssm.DescribeParametersInput{MaxResults: aws.Int32(50)}
	if nextToken != nil {
		input.NextToken = nextToken
	}
	if filter != "" {
		input.ParameterFilters = []ssmtypes.ParameterStringFilter{
			{Key: aws.String("Name"), Option: aws.String("Contains"), Values: []string{filter}},
		}
	}
	out, err := c.client.DescribeParameters(ctx, input)
	if err != nil {
		ilog.Error("SSM: ListParameters failed: %v", err)
		return nil, nil, err
	}
	var items []ParameterItem
	for _, p := range out.Parameters {
		items = append(items, ParameterItem{
			Name: aws.ToString(p.Name),
			Type: string(p.Type),
		})
	}
	ilog.Info("SSM: ListParameters returned %d items", len(items))
	return items, out.NextToken, nil
}

func (c *SSMClient) GetParameter(ctx context.Context, name string) (string, error) {
	out, err := c.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(name),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	if out.Parameter != nil && out.Parameter.Value != nil {
		return *out.Parameter.Value, nil
	}
	return "", nil
}

// BuildSessionCmd builds an *exec.Cmd for `aws ssm start-session`.
// If creds is provided, credentials are injected via env vars so the CLI
// does not re-prompt for MFA.
// Returns an error if session-manager-plugin is not installed.
func (c *SSMClient) BuildSessionCmd(creds *aws.Credentials, instanceID string) (*exec.Cmd, error) {
	ilog.Info("SSM: building session cmd for instance=%s profile=%s region=%s", instanceID, c.profile, c.region)
	if _, err := exec.LookPath("session-manager-plugin"); err != nil {
		ilog.Error("SSM: session-manager-plugin not found in PATH")
		return nil, fmt.Errorf("未找到 session-manager-plugin，请先安装：https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html")
	}

	args := []string{"ssm", "start-session", "--target", instanceID}
	env := os.Environ()

	if creds != nil && creds.SessionToken != "" {
		ilog.Info("SSM: injecting cached credentials via env vars (has SessionToken)")
		env = append(env,
			"AWS_ACCESS_KEY_ID="+creds.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY="+creds.SecretAccessKey,
			"AWS_SESSION_TOKEN="+creds.SessionToken,
		)
		if c.region != "" {
			env = append(env, "AWS_DEFAULT_REGION="+c.region, "AWS_REGION="+c.region)
		}
	} else {
		ilog.Info("SSM: no cached credentials, using --profile/--region flags")
		if c.region != "" {
			args = append(args, "--region", c.region)
		}
		if c.profile != "" && c.profile != "default" {
			args = append(args, "--profile", c.profile)
		}
	}

	ilog.Info("SSM: exec command: aws %v", args)
	cmd := exec.Command("aws", args...)
	cmd.Env = env
	return cmd, nil
}
