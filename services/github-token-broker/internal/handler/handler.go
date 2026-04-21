package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/GilmanLab/platform/services/github-token-broker/internal/broker"
)

// TokenBroker mints GitHub installation tokens.
type TokenBroker interface {
	Mint(ctx context.Context) (broker.Response, error)
}

// Handler adapts the token broker service to Lambda.
type Handler struct {
	broker TokenBroker
	logger *slog.Logger
}

// New constructs a Handler.
func New(tokenBroker TokenBroker, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return &Handler{
		broker: tokenBroker,
		logger: logger,
	}
}

// Handle rejects caller-selected input and returns a fixed-scope GitHub token.
func (h *Handler) Handle(ctx context.Context, payload json.RawMessage) (broker.Response, error) {
	if err := validateEmptyPayload(payload); err != nil {
		h.logger.Warn("rejected non-empty invocation payload")
		return broker.Response{}, err
	}

	response, err := h.broker.Mint(ctx)
	if err != nil {
		h.logger.Error("mint GitHub installation token", "error", err)
		return broker.Response{}, err
	}

	h.logger.Info("minted GitHub installation token", "repositories", response.Repositories, "expires_at", response.ExpiresAt)

	return response, nil
}

func validateEmptyPayload(payload json.RawMessage) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	return fmt.Errorf("github-token-broker does not accept invocation input")
}
