package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	ilog "github.com/shidaxi/iaws/internal/log"
)

type LambdaClient struct {
	client *lambda.Client
}

func NewLambda(cfg aws.Config) *LambdaClient {
	return &LambdaClient{client: lambda.NewFromConfig(cfg)}
}

type LambdaFunc struct {
	Name     string
	Runtime  string
	Handler  string
	Memory   int32
	Timeout  int32
	CodeSize int64
	Modified string
	Desc     string
}

func (c *LambdaClient) ListFunctions(ctx context.Context, marker *string, filter string) ([]LambdaFunc, *string, error) {
	ilog.Info("Lambda: ListFunctions (marker=%v filter=%q)", marker != nil, filter)
	if filter != "" {
		var items []LambdaFunc
		var nm *string
		for {
			input := &lambda.ListFunctionsInput{}
			if nm != nil {
				input.Marker = nm
			}
			out, err := c.client.ListFunctions(ctx, input)
			if err != nil {
				return nil, nil, err
			}
			for _, f := range out.Functions {
				item := buildLambdaFunc(f)
				if strings.Contains(strings.ToLower(item.Name), strings.ToLower(filter)) {
					items = append(items, item)
				}
			}
			if out.NextMarker == nil {
				break
			}
			nm = out.NextMarker
		}
		return items, nil, nil
	}
	input := &lambda.ListFunctionsInput{MaxItems: aws.Int32(50)}
	if marker != nil {
		input.Marker = marker
	}
	out, err := c.client.ListFunctions(ctx, input)
	if err != nil {
		return nil, nil, err
	}
	var items []LambdaFunc
	for _, f := range out.Functions {
		items = append(items, buildLambdaFunc(f))
	}
	return items, out.NextMarker, nil
}

func buildLambdaFunc(f lambdatypes.FunctionConfiguration) LambdaFunc {
	mem := int32(0)
	if f.MemorySize != nil {
		mem = *f.MemorySize
	}
	timeout := int32(0)
	if f.Timeout != nil {
		timeout = *f.Timeout
	}
	return LambdaFunc{
		Name:     aws.ToString(f.FunctionName),
		Runtime:  string(f.Runtime),
		Handler:  aws.ToString(f.Handler),
		Memory:   mem,
		Timeout:  timeout,
		CodeSize: f.CodeSize,
		Modified: aws.ToString(f.LastModified),
		Desc:     aws.ToString(f.Description),
	}
}

func FormatFuncDetail(f LambdaFunc) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name:       %s\n", f.Name)
	fmt.Fprintf(&b, "Runtime:    %s\n", f.Runtime)
	fmt.Fprintf(&b, "Handler:    %s\n", f.Handler)
	fmt.Fprintf(&b, "Memory:     %d MB\n", f.Memory)
	fmt.Fprintf(&b, "Timeout:    %d s\n", f.Timeout)
	fmt.Fprintf(&b, "Code Size:  %d KB\n", f.CodeSize/1024)
	fmt.Fprintf(&b, "Modified:   %s\n", f.Modified)
	if f.Desc != "" {
		fmt.Fprintf(&b, "Desc:       %s\n", f.Desc)
	}
	return b.String()
}
