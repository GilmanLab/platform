package githubbroker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

const (
	// DefaultFunctionName is the lab Lambda broker used to mint GitHub tokens.
	DefaultFunctionName = "glab-github-token-broker"
	// DefaultRegion is the lab fallback region when AWS config has no region.
	DefaultRegion = "us-west-2"

	invokePayload = "null"
	secretsRepo   = "GilmanLab/secrets"
	readPerm      = "read"
	contentsPerm  = "contents"
)

// LambdaInvoker is the subset of the AWS Lambda client used by Provider.
type LambdaInvoker interface {
	Invoke(
		ctx context.Context,
		params *lambda.InvokeInput,
		optFns ...func(*lambda.Options),
	) (*lambda.InvokeOutput, error)
}

// ClientFactory constructs an AWS Lambda invoker for a region.
type ClientFactory func(ctx context.Context, region string) (LambdaInvoker, error)

// Provider invokes the lab GitHub token broker Lambda.
type Provider struct {
	clientFactory ClientFactory
}

// NewProvider constructs a broker provider.
func NewProvider(clientFactory ClientFactory) Provider {
	if clientFactory == nil {
		clientFactory = NewAWSLambdaClient
	}

	return Provider{clientFactory: clientFactory}
}

// NewAWSLambdaClient constructs the real AWS Lambda client.
func NewAWSLambdaClient(ctx context.Context, region string) (LambdaInvoker, error) {
	var opts []func(*config.LoadOptions) error
	if strings.TrimSpace(region) != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for GitHub token broker: %w", err)
	}
	if cfg.Region == "" {
		cfg.Region = DefaultRegion
	}

	return lambda.NewFromConfig(cfg), nil
}

// Token invokes the broker and returns the GitHub token from its response.
func (p Provider) Token(ctx context.Context, request appsecrets.Request) (string, error) {
	functionName := request.BrokerFunction
	if strings.TrimSpace(functionName) == "" {
		functionName = DefaultFunctionName
	}

	client, err := p.clientFactory(ctx, request.AWSRegion)
	if err != nil {
		return "", err
	}

	output, err := client.Invoke(ctx, &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      []byte(invokePayload),
	})
	if err != nil {
		return "", fmt.Errorf("invoke GitHub token broker %s: %w", functionName, err)
	}
	if output.FunctionError != nil {
		return "", fmt.Errorf(
			"GitHub token broker %s returned function error %s: %s",
			functionName,
			*output.FunctionError,
			strings.TrimSpace(string(output.Payload)),
		)
	}

	token, err := parseResponse(output.Payload)
	if err != nil {
		return "", fmt.Errorf("decode GitHub token broker response: %w", err)
	}

	return token, nil
}

func parseResponse(payload []byte) (string, error) {
	var response struct {
		Token        string            `json:"token"`
		Repositories []string          `json:"repositories"`
		Permissions  map[string]string `json:"permissions"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", err
	}

	if strings.TrimSpace(response.Token) == "" {
		return "", errors.New("response did not include a token")
	}
	if !slices.Contains(response.Repositories, secretsRepo) {
		return "", fmt.Errorf("response did not include %s repository scope", secretsRepo)
	}
	if response.Permissions[contentsPerm] != readPerm {
		return "", fmt.Errorf("response did not include %s:%s permission", contentsPerm, readPerm)
	}

	return response.Token, nil
}
