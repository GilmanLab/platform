package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/GilmanLab/platform/services/github-token-broker/internal/broker"
	brokerconfig "github.com/GilmanLab/platform/services/github-token-broker/internal/config"
	"github.com/GilmanLab/platform/services/github-token-broker/internal/githubapp"
	"github.com/GilmanLab/platform/services/github-token-broker/internal/handler"
	"github.com/GilmanLab/platform/services/github-token-broker/internal/params"
)

func main() {
	cfg, err := brokerconfig.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	awsConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(cfg.AWSRegion))
	if err != nil {
		logger.Error("load AWS config", "error", err)
		os.Exit(1)
	}

	githubClient, err := githubapp.NewClient(http.DefaultClient, cfg.GitHubAPIBaseURL, nil)
	if err != nil {
		logger.Error("create GitHub client", "error", err)
		os.Exit(1)
	}

	appConfigStore := params.NewStore(ssm.NewFromConfig(awsConfig), params.Names{
		ClientID:       cfg.ClientIDParameter,
		InstallationID: cfg.InstallationIDParameter,
		PrivateKey:     cfg.PrivateKeyParameter,
	})
	tokenBroker := broker.NewService(appConfigStore, githubClient, githubapp.Target{
		Owner:      cfg.RepositoryOwner,
		Repository: cfg.RepositoryName,
		Permissions: map[string]string{
			"contents": "read",
		},
	})

	lambda.Start(handler.New(tokenBroker, logger).Handle)
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level

	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(io.Writer(os.Stdout), &slog.HandlerOptions{
		Level: slogLevel,
	}))
}
