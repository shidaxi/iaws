package config

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	ilog "github.com/shidaxi/iaws/internal/log"
)

// endpointResolver implements custom endpoint (e.g. LocalStack) for all services.
type endpointResolver struct {
	url string
}

func (e *endpointResolver) ResolveEndpoint(service, region string, options ...interface{}) (aws.Endpoint, error) {
	return aws.Endpoint{URL: e.url}, nil
}

// LoadOptions holds profile, region and optional custom endpoint (e.g. LocalStack).
type LoadOptions struct {
	Profile string
	Region  string
	// EndpointURL if set (e.g. http://localhost:4566) is used for all services (LocalStack).
	EndpointURL string
	// MFATokenProvider returns the MFA code when assuming a role with MFA. If nil, MFA code
	// is read from environment variable AWS_MFA_CODE.
	MFATokenProvider func() (string, error)
}

// assumeRoleTokenProvider 返回用于 assume role MFA 的 TokenProvider；优先使用 opts.MFATokenProvider，否则从环境变量 AWS_MFA_CODE 读取。
func assumeRoleTokenProvider(opts LoadOptions) func(*stscreds.AssumeRoleOptions) {
	return func(o *stscreds.AssumeRoleOptions) {
		o.TokenProvider = func() (string, error) {
			if opts.MFATokenProvider != nil {
				return opts.MFATokenProvider()
			}
			if code := os.Getenv("AWS_MFA_CODE"); code != "" {
				return code, nil
			}
			return "", fmt.Errorf("assume role with MFA enabled: cached credentials expired or missing — set AWS_MFA_CODE and run again (e.g. AWS_MFA_CODE=123456 ./iaws)")
		}
	}
}

// AWS holds loaded config and derived options.
type AWS struct {
	Config aws.Config
}

// Load loads AWS config from env/files using the given profile and region.
// If EndpointURL is set (e.g. from AWS_ENDPOINT_URL), it is used as custom endpoint for LocalStack.
// If the profile uses assume role with MFA, the MFA code is read from environment variable
// AWS_MFA_CODE (or the optional MFATokenProvider in LoadOptions). Set AWS_MFA_CODE before
// running iaws when using an MFA-protected role.
func Load(ctx context.Context, opts LoadOptions) (*AWS, error) {
	ilog.Info("loading AWS config: profile=%q region=%q", opts.Profile, opts.Region)
	var cfgOpts []func(*config.LoadOptions) error
	if opts.Profile != "" {
		cfgOpts = append(cfgOpts, config.WithSharedConfigProfile(opts.Profile))
	}
	if opts.Region != "" {
		cfgOpts = append(cfgOpts, config.WithRegion(opts.Region))
	}
	// 为 assume role + MFA 提供 TokenProvider，避免 "AssumeRoleTokenProvider session option not set" 错误
	cfgOpts = append(cfgOpts, config.WithAssumeRoleCredentialOptions(assumeRoleTokenProvider(opts)))
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		ilog.Error("load aws config failed: %v", err)
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	endpoint := opts.EndpointURL
	if endpoint == "" {
		endpoint = os.Getenv("AWS_ENDPOINT_URL")
	}
	if endpoint != "" {
		ilog.Info("using custom endpoint: %s", endpoint)
		cfg.EndpointResolverWithOptions = &endpointResolver{url: endpoint}
	}
	// 磁盘缓存 assume-role 凭证，一次 MFA 后在有效期内复用，无需每次输入
	cfg.Credentials = newFileCacheProvider(cfg.Credentials, opts.Profile, cfg.Region)
	ilog.Info("AWS config loaded: region=%s", cfg.Region)
	return &AWS{Config: cfg}, nil
}
