// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// maxProcessorResponseBytes bounds the processor response the server reads.
const maxProcessorResponseBytes = 1 << 20

// processorClientInterface calls the processor endpoint of an authorization details type.
type processorClientInterface interface {
	Process(ctx context.Context, cfg providers.AuthorizationDetailProcessor, req processorRequest) (*processorResponse,
		error)
}

// processorClient posts details to a processor endpoint. Redirects are not followed, so the details
// and credentials reach only the configured endpoint.
type processorClient struct {
	client httpservice.HTTPClientInterface
}

// newProcessorClient creates a new processorClient whose calls are bounded by timeout.
func newProcessorClient(timeout time.Duration) processorClientInterface {
	return &processorClient{client: httpservice.NewHTTPClientWithoutRedirects(timeout, false)}
}

// Process posts a batch of details to the processor; any failure is reported as errProcessorUnavailable.
func (c *processorClient) Process(ctx context.Context, cfg providers.AuthorizationDetailProcessor,
	req processorRequest) (*processorResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode processor request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errProcessorUnavailable, err)
	}
	httpReq.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	httpReq.Header.Set(serverconst.AcceptHeaderName, serverconst.ContentTypeJSON)
	if err := authenticate(ctx, cfg, httpReq); err != nil {
		return nil, fmt.Errorf("%w: %w", errProcessorUnavailable, err)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errProcessorUnavailable, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status %d", errProcessorUnavailable, resp.StatusCode)
	}

	// One byte past the limit is read so that an oversized response fails instead of being cut short.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProcessorResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read response body: %w", errProcessorUnavailable, err)
	}
	if len(raw) > maxProcessorResponseBytes {
		return nil, fmt.Errorf("%w: response body exceeds %d bytes", errProcessorUnavailable,
			maxProcessorResponseBytes)
	}
	var decoded processorResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("%w: invalid response body: %w", errProcessorUnavailable, err)
	}
	return &decoded, nil
}

// authenticate applies a processor's outbound authentication to a request.
func authenticate(ctx context.Context, cfg providers.AuthorizationDetailProcessor, request *http.Request) error {
	config, err := outboundauth.FromProperties(cfg.AuthenticationProperties)
	if err != nil {
		return fmt.Errorf("failed to read the processor authentication: %w", err)
	}
	authenticator, err := httpauth.New(config)
	if err != nil {
		return fmt.Errorf("invalid processor authentication: %w", err)
	}
	return authenticator.Authenticate(ctx, request)
}
