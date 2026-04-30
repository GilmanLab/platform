package githubbroker

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appsecrets "github.com/gilmanlab/platform/tools/labctl/internal/app/secrets"
)

func TestProviderTokenInvokesBroker(t *testing.T) {
	invoker := &fakeLambdaInvoker{
		output: &lambda.InvokeOutput{
			Payload: []byte(
				`{"token":"ghs_test","repositories":["GilmanLab/secrets"],"permissions":{"contents":"read"}}`,
			),
		},
	}
	provider := NewProvider(func(_ context.Context, region string) (LambdaInvoker, error) {
		assert.Equal(t, "us-west-2", region)

		return invoker, nil
	})

	token, err := provider.Token(context.Background(), appsecrets.Request{
		BrokerFunction: "custom-broker",
		AWSRegion:      "us-west-2",
	})

	require.NoError(t, err)
	assert.Equal(t, "ghs_test", token)
	require.NotNil(t, invoker.input)
	assert.Equal(t, "custom-broker", *invoker.input.FunctionName)
	assert.JSONEq(t, "null", string(invoker.input.Payload))
}

func TestProviderTokenValidatesBrokerResponse(t *testing.T) {
	tests := []struct {
		name      string
		payload   string
		wantError string
	}{
		{
			name:      "missing token",
			payload:   `{"repositories":["GilmanLab/secrets"],"permissions":{"contents":"read"}}`,
			wantError: "token",
		},
		{
			name:      "wrong repository",
			payload:   `{"token":"ghs_test","repositories":["Other/repo"],"permissions":{"contents":"read"}}`,
			wantError: "repository scope",
		},
		{
			name:      "wrong permission",
			payload:   `{"token":"ghs_test","repositories":["GilmanLab/secrets"],"permissions":{"contents":"write"}}`,
			wantError: "contents:read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewProvider(func(context.Context, string) (LambdaInvoker, error) {
				return &fakeLambdaInvoker{
					output: &lambda.InvokeOutput{Payload: []byte(tt.payload)},
				}, nil
			})

			_, err := provider.Token(context.Background(), appsecrets.Request{})

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

type fakeLambdaInvoker struct {
	input  *lambda.InvokeInput
	output *lambda.InvokeOutput
	err    error
}

func (f *fakeLambdaInvoker) Invoke(
	_ context.Context,
	input *lambda.InvokeInput,
	_ ...func(*lambda.Options),
) (*lambda.InvokeOutput, error) {
	f.input = input
	if f.err != nil {
		return nil, f.err
	}

	return f.output, nil
}
