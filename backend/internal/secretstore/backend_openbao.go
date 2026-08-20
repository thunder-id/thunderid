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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package secretstore

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
	"os"
	"strings"
	"time"
)

// KVTypeOpenBao names the OpenBao implementation in configuration.
const KVTypeOpenBao = "openbao"

const (
	// defaultOpenBaoMount is where a KV version 2 engine is mounted unless configured otherwise.
	defaultOpenBaoMount = "secret"
	// defaultOpenBaoTimeout bounds one call to the vault.
	defaultOpenBaoTimeout = 10 * time.Second
	// defaultOpenBaoCacheTTL is how long secrets read from the vault are reused.
	//
	// A control plane's write reaches one instance, which applies it at once. Every other instance of
	// the deployment picks it up by reloading, so this is what bounds how long one of them keeps
	// serving a credential that has since been regenerated: until it reloads, a login against that
	// credential fails there.
	//
	// Ten minutes trades that window against polling the vault for a credential that changes rarely.
	// Lower it with kv.cache_ttl_seconds where the window matters more than the traffic.
	defaultOpenBaoCacheTTL = 10 * time.Minute
	// openBaoTokenHeader carries the vault token.
	openBaoTokenHeader = "X-Vault-Token" // #nosec G101 -- a header name, not a credential
	// openBaoNamespaceHeader selects a namespace, for the deployments that use them.
	openBaoNamespaceHeader = "X-Vault-Namespace"
)

func init() {
	RegisterKV(KVTypeOpenBao, newOpenBaoBackend)
}

// openBaoBackend keeps secrets in an OpenBao key vault, over its KV version 2 engine.
//
// This is what several instances of one deployment share: each writes and reads the same paths, so a
// secret pushed to whichever instance the control plane reached is visible to all of them.
type openBaoBackend struct {
	address   string
	mount     string
	prefix    string
	namespace string
	token     string
	cacheTTL  time.Duration
	http      *http.Client
}

// newOpenBaoBackend builds a backend from configuration, failing on anything it cannot work without.
func newOpenBaoBackend(cfg KVConfig) (Backend, error) {
	address := strings.TrimRight(strings.TrimSpace(cfg.Address), "/")
	if address == "" {
		return nil, fmt.Errorf("key vault %q requires kv.address", KVTypeOpenBao)
	}
	// Without a token every call is rejected, which would surface as a store that is mysteriously
	// empty rather than as a configuration mistake.
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("key vault %q requires a token, from the %s environment variable or kv.token",
			KVTypeOpenBao, EnvKVToken)
	}

	mount := strings.Trim(strings.TrimSpace(cfg.Mount), "/")
	if mount == "" {
		mount = defaultOpenBaoMount
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultOpenBaoTimeout
	}
	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultOpenBaoCacheTTL
	}

	client, err := openBaoHTTPClient(cfg.CAFile, timeout)
	if err != nil {
		return nil, err
	}
	return &openBaoBackend{
		address:   address,
		mount:     mount,
		prefix:    strings.Trim(strings.TrimSpace(cfg.PathPrefix), "/"),
		namespace: strings.TrimSpace(cfg.Namespace),
		token:     strings.TrimSpace(cfg.Token),
		cacheTTL:  cacheTTL,
		http:      client,
	}, nil
}

// openBaoHTTPClient builds the client, trusting an extra certificate authority when one is named.
func openBaoHTTPClient(caFile string, timeout time.Duration) (*http.Client, error) {
	if strings.TrimSpace(caFile) == "" {
		return &http.Client{Timeout: timeout}, nil
	}

	// #nosec G304 -- the path is deployment configuration, not anything a request supplies.
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read the key vault certificate %s: %w", caFile, err)
	}
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s holds no certificate", caFile)
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}},
	}, nil
}

// Name identifies the vault and the path it serves, without the token.
func (b *openBaoBackend) Name() string {
	return fmt.Sprintf("%s:%s/%s", KVTypeOpenBao, b.address, b.dataPath(""))
}

func (b *openBaoBackend) CacheTTL() time.Duration { return b.cacheTTL }

// dataPath builds the KV version 2 path a secret's value is read and written at.
func (b *openBaoBackend) dataPath(name string) string {
	return b.enginePath("data", name)
}

// metadataPath builds the KV version 2 path a secret's metadata is listed and deleted at.
func (b *openBaoBackend) metadataPath(name string) string {
	return b.enginePath("metadata", name)
}

// enginePath assembles mount/segment/prefix/name, escaping the name so one containing a separator
// cannot reach outside this deployment's prefix.
func (b *openBaoBackend) enginePath(segment, name string) string {
	parts := []string{b.mount, segment}
	if b.prefix != "" {
		parts = append(parts, b.prefix)
	}
	if name != "" {
		parts = append(parts, url.PathEscape(name))
	}
	return strings.Join(parts, "/")
}

// Load reads every secret this deployment holds.
//
// The vault lists names and returns values separately, so this is a list followed by one read per
// name. It runs at startup and then once per cache TTL, not per request.
func (b *openBaoBackend) Load(ctx context.Context) (map[string]Secret, error) {
	names, err := b.list(ctx)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]Secret, len(names))
	for _, name := range names {
		secret, found, err := b.read(ctx, name)
		if err != nil {
			return nil, err
		}
		// A name listed but no longer readable was deleted between the two calls. Skipping it is
		// right: it is gone, and failing the whole load over it would drop every other secret too.
		if !found {
			continue
		}
		secrets[name] = secret
	}
	return secrets, nil
}

// Put stores a secret, replacing any entry of the same name.
func (b *openBaoBackend) Put(ctx context.Context, secret Secret) error {
	data, err := secretToVaultData(secret)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"data": data}
	return b.call(ctx, http.MethodPost, b.dataPath(secret.Name), body, nil)
}

// Delete removes every version of a secret. Removing an absent name is not an error.
func (b *openBaoBackend) Delete(ctx context.Context, name string) error {
	err := b.call(ctx, http.MethodDelete, b.metadataPath(name), nil, nil)
	if isVaultNotFound(err) {
		return nil
	}
	return err
}

// list returns the names held under this deployment's prefix.
func (b *openBaoBackend) list(ctx context.Context) ([]string, error) {
	var out struct {
		Data struct {
			Keys []string `json:"keys"`
		} `json:"data"`
	}
	// A prefix that holds nothing yet is a 404, which is an empty store rather than a failure.
	if err := b.call(ctx, "LIST", b.metadataPath(""), nil, &out); err != nil {
		if isVaultNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(out.Data.Keys))
	for _, key := range out.Data.Keys {
		// A key ending in "/" is a nested folder, not a secret. Nothing here writes one, so it belongs
		// to something else sharing the prefix and is left alone.
		if strings.HasSuffix(key, "/") {
			continue
		}
		name, err := url.PathUnescape(key)
		if err != nil {
			name = key
		}
		names = append(names, name)
	}
	return names, nil
}

// read returns one secret. The boolean reports whether the vault holds it.
func (b *openBaoBackend) read(ctx context.Context, name string) (Secret, bool, error) {
	var out struct {
		Data struct {
			Data json.RawMessage `json:"data"`
		} `json:"data"`
	}
	if err := b.call(ctx, http.MethodGet, b.dataPath(name), nil, &out); err != nil {
		if isVaultNotFound(err) {
			return Secret{}, false, nil
		}
		return Secret{}, false, err
	}
	// A soft-deleted secret still lists and still reads, but with no data.
	if len(out.Data.Data) == 0 || string(out.Data.Data) == "null" {
		return Secret{}, false, nil
	}

	secret, err := vaultDataToSecret(name, out.Data.Data)
	if err != nil {
		return Secret{}, false, err
	}
	return secret, true, nil
}

// call performs one request against the vault, decoding the response into out when given.
func (b *openBaoBackend) call(ctx context.Context, method, path string, body, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode the key vault request: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, b.address+"/v1/"+path, reader)
	if err != nil {
		return fmt.Errorf("failed to build the key vault request: %w", err)
	}
	req.Header.Set(openBaoTokenHeader, b.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if b.namespace != "" {
		req.Header.Set(openBaoNamespaceHeader, b.namespace)
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return fmt.Errorf("key vault request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return errVaultNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// The body carries the vault's own errors array, which names the actual problem (a denied
		// path, an expired token). It holds no secret: this is the response to a rejected request.
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("key vault returned %d for %s: %s",
			resp.StatusCode, path, strings.TrimSpace(string(detail)))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode the key vault response: %w", err)
	}
	return nil
}

// errVaultNotFound marks a path the vault does not hold, which several callers treat as empty rather
// than as a failure.
var errVaultNotFound = errors.New("not found in the key vault")

func isVaultNotFound(err error) bool { return errors.Is(err, errVaultNotFound) }

// vaultSecret is a secret as it is laid out in the vault.
//
// The fields are spelled out rather than being the Secret struct verbatim so that what an operator
// sees in the vault stays stable if Secret is ever reshaped, and so the hash parameters are flat
// enough to read there.
type vaultSecret struct {
	Kind        string `json:"kind"`
	Value       string `json:"value"`
	Algorithm   string `json:"algorithm,omitempty"`
	Salt        string `json:"salt,omitempty"`
	Iterations  int    `json:"iterations,omitempty"`
	KeySize     int    `json:"keySize,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Parallelism int    `json:"parallelism,omitempty"`
	Description string `json:"description,omitempty"`
}

// secretToVaultData converts a secret into what is written at its path.
func secretToVaultData(secret Secret) (map[string]interface{}, error) {
	v := vaultSecret{
		Kind:        string(secret.Kind),
		Value:       secret.Value,
		Algorithm:   secret.Algorithm,
		Salt:        secret.Parameters.Salt,
		Iterations:  secret.Parameters.Iterations,
		KeySize:     secret.Parameters.KeySize,
		Memory:      secret.Parameters.Memory,
		Parallelism: secret.Parameters.Parallelism,
		Description: secret.Description,
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to encode secret %s: %w", secret.Name, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to encode secret %s: %w", secret.Name, err)
	}
	return data, nil
}

// vaultDataToSecret converts what is stored at a path back into a secret.
//
// A secret written by hand may carry only a value, with no kind. It is read as a readable value,
// which is the only thing it can be: a hash with no algorithm could never be verified against.
func vaultDataToSecret(name string, raw json.RawMessage) (Secret, error) {
	var v vaultSecret
	if err := json.Unmarshal(raw, &v); err != nil {
		return Secret{}, fmt.Errorf("failed to read secret %s from the key vault: %w", name, err)
	}

	kind := Kind(v.Kind)
	if kind == "" {
		kind = KindValue
	}
	secret := Secret{
		Name:        name,
		Kind:        kind,
		Value:       v.Value,
		Algorithm:   v.Algorithm,
		Description: v.Description,
	}
	secret.Parameters = HashParameters{
		Salt:        v.Salt,
		Iterations:  v.Iterations,
		KeySize:     v.KeySize,
		Memory:      v.Memory,
		Parallelism: v.Parallelism,
	}
	return secret, nil
}
