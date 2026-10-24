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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
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
	// HeldValues reports which of the named variables and secrets the gateway's store holds a value
	// for, and when each was last set.
	HeldValues(ctx context.Context, gw *Gateway, key string, variables, secrets []string) (*HeldValues, error)
}

// HeldValues are the values a gateway's store holds, by name, each with when it was last set, zero
// when the store did not say.
type HeldValues struct {
	Variables map[string]time.Time
	Secrets   map[string]time.Time
}

// missing returns the names among those asked for that the store does not hold.
func (h *HeldValues) missing(variables, secrets []string) *MissingValues {
	return &MissingValues{Variables: unheld(variables, h.Variables), Secrets: unheld(secrets, h.Secrets)}
}

// heldTimes is when each held value was last set, by collection and name, as recorded at an apply.
type heldTimes struct {
	Variables map[string]time.Time `json:"variables,omitempty"`
	Secrets   map[string]time.Time `json:"secrets,omitempty"`
}

// snapshot records when each of the named values the store holds was last set, as the store said.
func (h *HeldValues) snapshot(variables, secrets []string) string {
	pick := func(names []string, held map[string]time.Time) map[string]time.Time {
		out := map[string]time.Time{}
		for _, name := range names {
			if set, ok := held[name]; ok && !set.IsZero() {
				out[name] = set.UTC()
			}
		}
		return out
	}
	recorded, err := json.Marshal(heldTimes{Variables: pick(variables, h.Variables), Secrets: pick(secrets, h.Secrets)})
	if err != nil {
		return ""
	}
	return string(recorded)
}

// changedFrom returns the names among those asked for whose value the store says was set at another
// time than the snapshot recorded. Only the store's own times are compared, never with another
// clock; a value the snapshot did not record, or whose time the store does not say, is not reported.
func (h *HeldValues) changedFrom(variables, secrets []string, snapshot string) *ChangedValues {
	changed := &ChangedValues{}
	var recorded heldTimes
	if snapshot == "" || json.Unmarshal([]byte(snapshot), &recorded) != nil {
		return changed
	}
	differs := func(name string, held, then map[string]time.Time) bool {
		set, ok := held[name]
		was, recordedThen := then[name]
		return ok && recordedThen && !set.IsZero() && !set.Equal(was)
	}
	for _, name := range variables {
		if differs(name, h.Variables, recorded.Variables) {
			changed.Variables = append(changed.Variables, name)
		}
	}
	for _, name := range secrets {
		if differs(name, h.Secrets, recorded.Secrets) {
			changed.Secrets = append(changed.Secrets, name)
		}
	}
	return changed
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

// unreachable is the error a failed call to a gateway is reported as: a host that resolves to an
// address this deployment does not reach gateways on is named as such, since the administrator can
// put that right in the configuration, and anything else as the gateway being unreachable.
func unreachable(err error) *tidcommon.ServiceError {
	if errors.Is(err, syshttp.ErrPrivateAddress) {
		return &ErrorGatewayPrivateAddress
	}
	return &ErrorGatewayUnreachable
}

// neverConnected reports whether a call failed before the request could be sent: the connection was
// refused or never made, the dialer refused the gateway's private address, or the gateway's
// certificate did not verify.
func neverConnected(err error) bool {
	if errors.Is(err, syshttp.ErrPrivateAddress) {
		return true
	}
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
// no redirect: a redirect would hand the key's header to whatever the response pointed at. Unless the
// deployment allows gateways on private addresses, a host that resolves to a loopback, link-local or
// private address is refused as it is dialed, so a registered name cannot be pointed at something
// inside this plane's network after the fact.
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
	return syshttp.NewHTTPClientWithRootCAs(timeout, roots, !allowPrivateAddresses()), nil
}

// releaseConnections closes a client's idle connections. A client is built per gateway call, for the
// gateway's certificate authority, so nothing would reuse them.
func releaseConnections(client syshttp.HTTPClientInterface) {
	if closer, ok := client.(interface{ CloseIdleConnections() }); ok {
		closer.CloseIdleConnections()
	}
}
