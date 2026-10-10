// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
)

// gatewayImportRequest is the body of a gateway's POST /import.
type gatewayImportRequest struct {
	Content   string                 `json:"content"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DryRun    bool                   `json:"dryRun,omitempty"`
	Options   gatewayImportOptions   `json:"options"`
	Deletions []gatewayDeletion      `json:"deletions,omitempty"`
}

type gatewayImportOptions struct {
	Upsert          bool   `json:"upsert"`
	ContinueOnError bool   `json:"continueOnError"`
	Target          string `json:"target"`
}

// gatewayDeletion asks the gateway to remove one resource by id.
type gatewayDeletion struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
}

// errGatewayRefused is a gateway answering an import with something other than success.
var errGatewayRefused = errors.New("the gateway refused the import")

// errImportNotWritten marks an import the gateway is known to have written nothing of: it was never
// sent, the connection to the gateway was never made, or the gateway refused it whole with a 4xx,
// which it answers before writing anything. Any other failure, such as a timeout or a connection
// lost after the request went out, leaves open whether the gateway wrote it.
var errImportNotWritten = errors.New("the gateway wrote nothing of the import")

// gatewayClientInterface calls a gateway's own APIs.
type gatewayClientInterface interface {
	// Import posts to the gateway's /import presenting its key, and returns the answer as it came.
	Import(ctx context.Context, gw *Gateway, key string, req gatewayImportRequest) (json.RawMessage, error)
	// UnsetValues returns which of the named variables and secrets the gateway's store holds no value
	// for.
	UnsetValues(ctx context.Context, gw *Gateway, key string, variables, secrets []string) (*MissingValues, error)
}

// gatewayClient reaches a gateway over HTTPS.
type gatewayClient struct{}

func newGatewayClient() *gatewayClient {
	return &gatewayClient{}
}

func (c *gatewayClient) Import(ctx context.Context, gw *Gateway, key string,
	req gatewayImportRequest) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to encode the import: %w", errImportNotWritten, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, gw.BaseURL+"/import", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: failed to build the import request: %w", errImportNotWritten, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The key is presented in its own header, which is what the gateway's management API key
	// authenticator reads, and is trusted for the import API alone.
	httpReq.Header.Set(constants.APIKeyHeaderName, key)

	client, err := httpClientFor(gw, importTimeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errImportNotWritten, err)
	}
	defer releaseConnections(client)
	resp, err := client.Do(httpReq)
	if err != nil {
		if neverConnected(err) {
			return nil, fmt.Errorf("%w: failed to reach the gateway: %w", errImportNotWritten, err)
		}
		return nil, fmt.Errorf("failed to reach the gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxImportAnswer))
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's answer: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError {
		return answer, fmt.Errorf("%w: %w: status %d", errGatewayRefused, errImportNotWritten, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return answer, fmt.Errorf("%w: status %d", errGatewayRefused, resp.StatusCode)
	}
	return answer, nil
}

// neverConnected reports whether a call failed before the request could be sent: the connection was
// refused or never made, or the gateway's certificate did not verify.
func neverConnected(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}
	var certErr *tls.CertificateVerificationError
	return errors.As(err, &certErr)
}

// storeTimeout bounds one call to a gateway's store, which reads or writes a single value or a page.
const storeTimeout = 30 * time.Second

// maxImportAnswer bounds how much of a gateway's answer is read, so a misbehaving one cannot exhaust
// this plane's memory.
const maxImportAnswer = 16 << 20

// importTimeout bounds one apply. An import writes every resource of a version, which takes longer
// than an ordinary call.
const importTimeout = 2 * time.Minute

// httpClientFor builds a client trusting the system roots and, when the gateway names one, its own
// certificate authority. Naming one keeps verification on for a gateway with a private certificate,
// rather than turning it off.
//
// Every call carries the gateway's key, so the client reaches the gateway only over HTTPS and follows
// no redirect: a redirect would hand the key's header to whatever the response pointed at.
func httpClientFor(gw *Gateway, timeout time.Duration) (syshttp.HTTPClientInterface, error) {
	if !strings.HasPrefix(strings.ToLower(gw.BaseURL), "https://") {
		return nil, fmt.Errorf("the gateway's baseUrl is not https, so its key is not sent")
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if gw.CACertificate != "" && !roots.AppendCertsFromPEM([]byte(gw.CACertificate)) {
		return nil, fmt.Errorf("the gateway's caCertificate is not a PEM certificate")
	}
	return syshttp.NewHTTPClientWithRootCAs(timeout, roots), nil
}

// releaseConnections closes a client's idle connections. A client is built per gateway call, for the
// gateway's certificate authority, so nothing would reuse them.
func releaseConnections(client syshttp.HTTPClientInterface) {
	if closer, ok := client.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
