/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package secretcapture routes a credential the control plane creates to the data plane that holds
// it. No control plane keeps one: credentials live in each data plane's own store.
package secretcapture

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	envmgrservice "github.com/thunder-id/thunderid/internal/envmgr/service"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

// secretForwarder sends a captured credential straight to the Data Plane's secret service instead of
// keeping it on the Control Plane.
//
// Propagation is immediate rather than part of a configuration promotion: a credential that arrives
// late is a credential that rejects logins until it lands. Nothing is persisted here, so the Control
// Plane holds no secret at rest.
type secretForwarder struct {
	baseURL string
	token   string
	http    *http.Client
	// hashConfig reads the server's configured hashing. It is a field so a test can supply one without
	// a server runtime.
	hashConfig func() (cryptolib.HashConfig, error)
}

// newSecretForwarder builds a forwarder for the configured environment manager. It returns nil when
// none is configured, which leaves this server with nowhere to put a captured credential.
//
// The target is the environment manager rather than a secret provider directly: this server hosts every
// tenant, so it cannot know which data plane's provider a captured credential belongs on. The
// environment manager holds that mapping and routes by tenant.
func newSecretForwarder(cfg config.Config, readHashConfig HashConfigReader) *secretForwarder {
	sp := cfg.Server.SecurityConfig.SecretProvider.Service
	if strings.TrimSpace(sp.URL) == "" {
		return nil
	}
	timeout := time.Duration(sp.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &secretForwarder{
		baseURL:    strings.TrimRight(sp.URL, "/"),
		token:      sp.Token,
		http:       &http.Client{Timeout: timeout},
		hashConfig: readHashConfig,
	}
}

// verifiableFields are the credentials that are only ever checked against something a caller presents.
// Those are forwarded as a hash, because nothing needs the original. Every other credential, such as a
// Vonage API secret or an SMS gateway key, has to be replayed to a third party and is forwarded as a
// readable value.
var verifiableFields = map[string]bool{
	"clientsecret": true,
	"password":     true,
	"flowsecret":   true,
}

// CaptureSecret forwards the credential to the secret service under its declarative placeholder key.
// It is best-effort: a failure is logged and never propagated, matching the local capturer, so creating
// a resource does not fail because the secret service is briefly unavailable.
func (f *secretForwarder) CaptureSecret(ctx context.Context, resourceType, resourceName, field, value string) {
	if value == "" {
		return
	}

	f.capture(ctx, resourceType, resourceName, field, value, verifiableFields[strings.ToLower(field)])
}

// CaptureReplayableSecret forwards a credential that has to stay readable because the Data Plane hands
// it to a third party, such as a connection's client secret sent to the upstream provider. The field
// name alone cannot decide this: a connection's ClientSecret is replayed, while an application's
// ClientSecret of the same name is only ever verified.
func (f *secretForwarder) CaptureReplayableSecret(ctx context.Context, resourceType, resourceName, field,
	value string) {
	if value == "" {
		return
	}
	f.capture(ctx, resourceType, resourceName, field, value, false)
}

// capture builds the body for the chosen representation and writes it to the secret service.
func (f *secretForwarder) capture(ctx context.Context, resourceType, resourceName, field, value string,
	verifiable bool) {
	key := varname.DeriveVariableName(resourceType, resourceName, field)
	body, err := f.buildBody(value, verifiable, fmt.Sprintf("Captured %s for %s", field, resourceName))
	if err != nil {
		log.GetLogger().Warn(ctx, "Failed to prepare a secret for the secret service",
			log.String("key", key), log.Error(err))
		return
	}
	if err := f.put(ctx, key, body); err != nil {
		log.GetLogger().Warn(ctx, "Failed to forward a secret to the secret service",
			log.String("key", key), log.Error(err))
	}
}

// putSecretBody is the write payload of the secret service.
type putSecretBody struct {
	Kind        string          `json:"kind"`
	Value       string          `json:"value"`
	Algorithm   string          `json:"algorithm,omitempty"`
	Parameters  *hashParameters `json:"parameters,omitempty"`
	Description string          `json:"description,omitempty"`
}

type hashParameters struct {
	Salt        string `json:"salt,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`
	KeySize     int    `json:"keySize,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Parallelism int    `json:"parallelism,omitempty"`
}

// buildBody decides how the credential is held and, for a verifiable one, hashes it here so the
// plaintext never leaves this process. The hash comes from the server's own configured hashing, so what
// is stored is exactly what a local write would have produced.
func (f *secretForwarder) buildBody(value string, verifiable bool, description string) (putSecretBody, error) {
	if !verifiable {
		return putSecretBody{Kind: "value", Value: value, Description: description}, nil
	}

	hashed, err := hashCredential(f.hashConfig, value)
	if err != nil {
		return putSecretBody{}, err
	}

	return putSecretBody{
		Kind:      "hash",
		Value:     hashed.Hash,
		Algorithm: string(hashed.Algorithm),
		Parameters: &hashParameters{
			Salt:        hashed.Parameters.Salt,
			Iterations:  hashed.Parameters.Iterations,
			KeySize:     hashed.Parameters.KeySize,
			Memory:      hashed.Parameters.Memory,
			Parallelism: hashed.Parameters.Parallelism,
		},
		Description: description,
	}, nil
}

// hashCredential hashes a credential with the server's own configured hashing, so what is stored is
// exactly what a local write would have produced. readHashConfig is a parameter so a test can supply a
// configuration without a server runtime.
func hashCredential(readHashConfig HashConfigReader, value string) (cryptolib.Credential, error) {
	if readHashConfig == nil {
		return cryptolib.Credential{}, fmt.Errorf("no password hashing configuration was supplied")
	}
	hashCfg, err := readHashConfig()
	if err != nil {
		return cryptolib.Credential{}, err
	}
	hashService, err := cryptolib.Initialize(hashCfg)
	if err != nil {
		return cryptolib.Credential{}, fmt.Errorf("failed to initialize hashing: %w", err)
	}
	hashed, err := hashService.Generate([]byte(value))
	if err != nil {
		return cryptolib.Credential{}, fmt.Errorf("failed to hash the credential: %w", err)
	}
	return hashed, nil
}

// put writes one secret to the secret service.
func (f *secretForwarder) put(ctx context.Context, key string, body putSecretBody) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to encode the secret: %w", err)
	}

	tenant := deployment.Resolve(ctx, "")
	if strings.TrimSpace(tenant) == "" {
		return fmt.Errorf("no tenant in context, so the secret cannot be routed")
	}
	url := f.baseURL + "/tenants/" + tenant + "/secrets/" + key
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("failed to build the request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}

	resp, err := f.http.Do(req)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("secret service returned %d", resp.StatusCode)
	}
	return nil
}

// HashConfigReader reads the server's password-hashing configuration. It is supplied by the caller
// because that configuration is the server's, and each plane builds it for itself.
type HashConfigReader func() (cryptolib.HashConfig, error)

// HasherFor lets the environment manager store a credential that is only ever verified, such as a
// client secret set by hand from the console. The hash is produced here rather than there so a
// credential set by an operator is indistinguishable from one this server captured.
func HasherFor(readHashConfig HashConfigReader) func(string) (envmgrservice.HashedSecret, error) {
	return func(value string) (envmgrservice.HashedSecret, error) {
		hashed, err := hashCredential(readHashConfig, value)
		if err != nil {
			return envmgrservice.HashedSecret{}, err
		}
		return envmgrservice.HashedSecret{
			Hash:        hashed.Hash,
			Algorithm:   string(hashed.Algorithm),
			Salt:        hashed.Parameters.Salt,
			Iterations:  hashed.Parameters.Iterations,
			KeySize:     hashed.Parameters.KeySize,
			Memory:      hashed.Parameters.Memory,
			Parallelism: hashed.Parameters.Parallelism,
		}, nil
	}
}

// Capturer is the hook the resource services call after creating a credential, so it reaches the data
// plane that holds it.
type Capturer interface {
	CaptureSecret(ctx context.Context, resourceType, resourceName, field, value string)
	CaptureReplayableSecret(ctx context.Context, resourceType, resourceName, field, value string)
}

// unroutedSecretCapture stands in when no secret service is configured. The Control Plane keeps no
// secret at rest, so a captured credential has nowhere to go; it names the placeholder in a warning
// so the credential can be set by hand instead of surfacing as a missing variable at import time.
type unroutedSecretCapture struct{}

func (unroutedSecretCapture) CaptureSecret(ctx context.Context, resourceType, resourceName, field,
	value string) {
	if value == "" {
		return
	}
	log.GetLogger().Warn(ctx, "No secret service is configured, so a captured secret was discarded",
		log.String("key", varname.DeriveVariableName(resourceType, resourceName, field)))
}

func (c unroutedSecretCapture) CaptureReplayableSecret(ctx context.Context, resourceType, resourceName,
	field, value string) {
	c.CaptureSecret(ctx, resourceType, resourceName, field, value)
}

// Select sends a captured credential to the Data Plane's secret service. Nothing is kept
// on this server, so without a configured service there is no store to fall back to.
//
// The environment manager running in this process is preferred over a configured URL: it knows the same
// deployment-to-data-plane mapping without a round trip, and it is the manager the console writes
// through, so both paths reach the same data plane.
func Select(ctx context.Context, logger *log.Logger, cfg config.Config,
	router LocalCaptureRouter, readHashConfig HashConfigReader) Capturer {
	if router != nil {
		logger.Info(ctx, "Captured secrets will be routed by the environment manager in this server")
		return &localSecretCapture{router: router, hashConfig: readHashConfig}
	}

	forwarder := newSecretForwarder(cfg, readHashConfig)
	if forwarder == nil {
		logger.Warn(ctx, "No secret service is configured, so captured secrets will be discarded")
		return unroutedSecretCapture{}
	}
	logger.Info(ctx, "Captured secrets will be forwarded to the secret service",
		log.String("url", forwarder.baseURL))
	return forwarder
}
