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
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
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

// gatewayClientInterface calls a gateway's own APIs.
type gatewayClientInterface interface {
	// Import posts to the gateway's /import presenting its key, and returns the answer as it came.
	Import(ctx context.Context, gw *Gateway, key string, req gatewayImportRequest) (json.RawMessage, error)
	// Held returns which of the named values the gateway's store holds in a collection, variables
	// or secrets.
	Held(ctx context.Context, gw *Gateway, key, collection string, names []string) (map[string]bool, error)
	// Forward sends a call to the gateway's variable and secret store and returns its answer as it came.
	Forward(ctx context.Context, gw *Gateway, key string, call StoreCall) (*StoreAnswer, error)
}

// StoreCall is a call to a gateway's variable and secret store, made on an administrator's behalf.
type StoreCall struct {
	Method string
	// Path is the store path on the gateway: /variables, /secrets, or one value under either.
	Path  string
	Query url.Values
	Body  []byte
}

// StoreAnswer is the gateway's answer to a StoreCall.
type StoreAnswer struct {
	Status      int
	ContentType string
	Body        []byte
}

// gatewayClient reaches a gateway over HTTPS.
type gatewayClient struct{}

func newGatewayClient() gatewayClientInterface {
	return &gatewayClient{}
}

func (c *gatewayClient) Import(ctx context.Context, gw *Gateway, key string,
	req gatewayImportRequest) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the import: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, gw.BaseURL+"/import", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build the import request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// The key is presented in its own header, which is what the gateway's management API key
	// authenticator reads, and is trusted for the import API alone.
	httpReq.Header.Set(constants.APIKeyHeaderName, key)

	client, err := httpClientFor(gw)
	if err != nil {
		return nil, err
	}
	// A client is built per gateway, for its certificate authority, so its connections are released
	// once the call is done rather than left idle.
	defer client.CloseIdleConnections()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to reach the gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxImportAnswer))
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return answer, fmt.Errorf("%w: status %d", errGatewayRefused, resp.StatusCode)
	}
	return answer, nil
}

// Store collections a gateway serves. A collection's listing holds its entries under the same name.
const (
	collectionVariables = "variables"
	collectionSecrets   = "secrets"
)

// maxNamesPerLookup matches the most names a gateway's store answers for in one call.
const maxNamesPerLookup = 100

func (c *gatewayClient) Held(ctx context.Context, gw *Gateway, key, collection string,
	names []string) (map[string]bool, error) {
	if collection != collectionVariables && collection != collectionSecrets {
		return nil, fmt.Errorf("unknown store collection %q", collection)
	}
	held := map[string]bool{}
	for start := 0; start < len(names); start += maxNamesPerLookup {
		batch := names[start:min(start+maxNamesPerLookup, len(names))]
		answer, err := c.Forward(ctx, gw, key, StoreCall{
			Method: http.MethodGet,
			Path:   "/" + collection,
			Query: url.Values{"names": {strings.Join(batch, ",")},
				"limit": {fmt.Sprint(maxNamesPerLookup)}},
		})
		if err != nil {
			return nil, err
		}
		if answer.Status != http.StatusOK {
			return nil, fmt.Errorf("%w: status %d", errGatewayRefused, answer.Status)
		}
		var listing map[string]json.RawMessage
		var entries []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(answer.Body, &listing); err != nil {
			return nil, fmt.Errorf("failed to read the gateway's %s: %w", collection, err)
		}
		if raw, ok := listing[collection]; ok {
			if err := json.Unmarshal(raw, &entries); err != nil {
				return nil, fmt.Errorf("failed to read the gateway's %s: %w", collection, err)
			}
		}
		for _, entry := range entries {
			held[entry.Name] = true
		}
	}
	return held, nil
}

func (c *gatewayClient) Forward(ctx context.Context, gw *Gateway, key string,
	call StoreCall) (*StoreAnswer, error) {
	target := gw.BaseURL + call.Path
	if len(call.Query) > 0 {
		target += "?" + call.Query.Encode()
	}
	var body io.Reader
	if call.Body != nil {
		body = bytes.NewReader(call.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, call.Method, target, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build the store request: %w", err)
	}
	if call.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set(constants.APIKeyHeaderName, key)

	client, err := httpClientFor(gw)
	if err != nil {
		return nil, err
	}
	defer client.CloseIdleConnections()
	client.Timeout = storeTimeout
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to reach the gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxImportAnswer))
	if err != nil {
		return nil, fmt.Errorf("failed to read the gateway's answer: %w", err)
	}
	return &StoreAnswer{Status: resp.StatusCode, ContentType: resp.Header.Get("Content-Type"), Body: answer}, nil
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
func httpClientFor(gw *Gateway) (*http.Client, error) {
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
	return &http.Client{
		Timeout: importTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			// #nosec G402 -- Min TLS version is TLS 1.2 or higher based on config
			TLSClientConfig: &tls.Config{
				MinVersion: syshttp.GetTLSVersion(config.GetServerRuntime().Config),
				RootCAs:    roots,
			},
		},
	}, nil
}
