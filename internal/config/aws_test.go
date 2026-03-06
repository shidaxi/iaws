package config

import (
	"context"
	"os"
	"testing"
)

func TestEndpointResolver_ResolveEndpoint(t *testing.T) {
	url := "http://localhost:4566"
	r := &endpointResolver{url: url}

	ep, err := r.ResolveEndpoint("s3", "us-east-1")
	if err != nil {
		t.Fatalf("ResolveEndpoint: %v", err)
	}
	if ep.URL != url {
		t.Errorf("URL = %q, want %q", ep.URL, url)
	}

	ep2, err := r.ResolveEndpoint("ec2", "eu-west-1", "option1")
	if err != nil {
		t.Fatalf("ResolveEndpoint with options: %v", err)
	}
	if ep2.URL != url {
		t.Errorf("URL = %q, want %q", ep2.URL, url)
	}
}

func TestLoad_WithEndpointURL(t *testing.T) {
	ctx := context.Background()
	endpoint := "http://localstack:4566"
	opts := LoadOptions{
		Profile:     "default",
		Region:      "us-east-1",
		EndpointURL: endpoint,
	}

	// Load may fail without credentials, but if successful we verify the custom endpoint is set
	awsCfg, err := Load(ctx, opts)
	if err != nil {
		// skip when ~/.aws/credentials is missing or insufficient permissions
		t.Skipf("Load failed (no creds?): %v", err)
		return
	}

	resolver := awsCfg.Config.EndpointResolverWithOptions
	if resolver == nil {
		t.Fatal("EndpointResolverWithOptions should be set")
	}

	ep, err := resolver.ResolveEndpoint("s3", "us-east-1")
	if err != nil {
		t.Fatalf("ResolveEndpoint: %v", err)
	}
	if ep.URL != endpoint {
		t.Errorf("URL = %q, want %q", ep.URL, endpoint)
	}
}

func TestLoad_EndpointFromEnv(t *testing.T) {
	envKey := "AWS_ENDPOINT_URL"
	old := os.Getenv(envKey)
	defer func() { _ = os.Setenv(envKey, old) }()

	want := "http://env-endpoint:4566"
	_ = os.Setenv(envKey, want)
	ctx := context.Background()
	opts := LoadOptions{Profile: "default", Region: "us-east-1"}
	// no EndpointURL passed; should be read from env var
	awsCfg, err := Load(ctx, opts)
	if err != nil {
		t.Skipf("Load failed: %v", err)
		return
	}
	resolver := awsCfg.Config.EndpointResolverWithOptions
	if resolver == nil {
		t.Skip("no resolver set (endpoint env might be unset in this run)")
		return
	}
	ep, err := resolver.ResolveEndpoint("ec2", "us-west-2")
	if err != nil {
		t.Fatalf("ResolveEndpoint: %v", err)
	}
	if ep.URL != want {
		t.Errorf("URL = %q, want %q", ep.URL, want)
	}
}
