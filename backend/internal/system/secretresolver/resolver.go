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

// Package secretresolver resolves secret references held in configuration.
//
// Configuration promoted from a Control Plane stores a reference such as "kv:MY_APP_CLIENT_SECRET"
// rather than the secret itself, so no secret leaves the Control Plane. This package turns such a
// reference into its value.
//
// Where it reads from depends on how the deployment holds its secrets. A store served by this same
// process is read per reference, so a credential regenerated on the Control Plane takes effect as
// soon as it lands. A standalone provider reached over HTTP is cached in memory, loaded once at
// startup and refreshed for a single name when a reference is not held, because every miss there
// costs an outbound call.
package secretresolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Prefix marks a configuration value as a reference to a secret rather than the secret itself.
const Prefix = "kv:"

// minRefreshInterval throttles single-name refreshes, so a reference that the provider does not hold
// cannot turn every request that reads it into an outbound call.
const minRefreshInterval = 30 * time.Second

// ErrNotConfigured is returned when a reference is met but no secret provider is configured.
var ErrNotConfigured = errors.New("a secret reference was found but no secret provider is configured")

// ErrSecretNotFound is returned when the provider does not hold the referenced secret.
var ErrSecretNotFound = errors.New("the referenced secret is not available from the secret provider")

// IsReference reports whether a stored configuration value is a secret reference.
func IsReference(value string) bool {
	return strings.HasPrefix(value, Prefix)
}

// ReferenceName returns the secret name a reference points at.
func ReferenceName(value string) string {
	return strings.TrimPrefix(value, Prefix)
}

// Config describes the secret provider to resolve against.
type Config struct {
	// BaseURL is the provider's base URL. Empty disables resolution.
	BaseURL string
	Token   string
	Timeout time.Duration
	// Local reads one secret from a store held in this process. It is set when this server serves its
	// own store: reading that over HTTP would mean the server authenticating to its own management
	// API, which it has no credentials to do. When set it is used in place of BaseURL.
	//
	// It is consulted on every resolution rather than cached here. The store it reads is already an
	// in-memory cache with its own freshness policy, so keeping a second copy would mean a credential
	// regenerated on the control plane kept being rejected until this process restarted.
	Local func(ctx context.Context, name string) (LocalSecret, bool, error)
}

// LocalSecret is one secret as a store in this process holds it.
type LocalSecret struct {
	Kind        string
	Value       string
	Algorithm   string
	Salt        string
	Iterations  int
	KeySize     int
	Memory      int
	Parallelism int
}

// providerSecret converts a locally held secret into the shape resolution works in.
func (l LocalSecret) providerSecret(name string) providerSecret {
	p := providerSecret{Name: name, Kind: l.Kind, Value: l.Value, Algorithm: l.Algorithm}
	p.Parameters.Salt = l.Salt
	p.Parameters.Iterations = l.Iterations
	p.Parameters.KeySize = l.KeySize
	p.Parameters.Memory = l.Memory
	p.Parameters.Parallelism = l.Parallelism
	return p
}

// Resolver resolves secret references against a secret provider, caching what it loads.
type Resolver struct {
	cfg  Config
	http *http.Client

	mu      sync.RWMutex
	secrets map[string]string
	// hashes holds the structured form of every hash backed secret, for verifying a presented
	// credential against it.
	hashes map[string]Hash
	// lastMiss records when a name was last fetched and not found, to throttle repeat lookups.
	lastMiss map[string]time.Time
}

// New builds a Resolver. It performs no I/O: call LoadAll to populate the cache.
func New(cfg Config) *Resolver {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Resolver{
		cfg:      cfg,
		http:     &http.Client{Timeout: timeout},
		secrets:  map[string]string{},
		hashes:   map[string]Hash{},
		lastMiss: map[string]time.Time{},
	}
}

// Enabled reports whether a secret provider is configured.
func (r *Resolver) Enabled() bool {
	if r == nil {
		return false
	}
	return r.cfg.Local != nil || strings.TrimSpace(r.cfg.BaseURL) != ""
}

// providerSecret is one entry as the secret provider serves it.
//
// A secret is held either as a hash, for a credential that is only ever verified, or as a readable
// value, for one that has to be replayed to a third party. Both arrive here; Resolve turns each into the
// form the configuration expects.
type providerSecret struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Algorithm  string `json:"algorithm,omitempty"`
	Parameters struct {
		Salt        string `json:"salt,omitempty"`
		Iterations  int    `json:"iterations,omitempty"`
		KeySize     int    `json:"keySize,omitempty"`
		Memory      int    `json:"memory,omitempty"`
		Parallelism int    `json:"parallelism,omitempty"`
	} `json:"parameters,omitempty"`
}

// kindHash marks a secret held as a one-way hash.
const kindHash = "hash"

// storedCredential mirrors the shape a declarative credential takes, so a hash can be handed to the
// importer unchanged and end up stored exactly as one written through the API.
type storedCredential struct {
	StorageType       string `json:"storageType"`
	StorageAlgo       string `json:"storageAlgo"`
	StorageAlgoParams struct {
		Salt        string `json:"salt,omitempty"`
		Iterations  int    `json:"iterations,omitempty"`
		KeySize     int    `json:"keySize,omitempty"`
		Memory      int    `json:"memory,omitempty"`
		Parallelism int    `json:"parallelism,omitempty"`
	} `json:"storageAlgoParams"`
	Value string `json:"value"`
}

// substitution returns the text that replaces a placeholder for this secret.
//
// A readable value is substituted as itself. A hash cannot be: the configuration expects a credential,
// and the original is unrecoverable. It is therefore rendered as the declarative credential form, a JSON
// array. YAML is a superset of JSON, so that text parses as a list of credential objects and the
// importer stores the hash as it stands rather than hashing it a second time.
func (p *providerSecret) substitution() (string, error) {
	if p.Kind != kindHash {
		return p.Value, nil
	}

	cred := storedCredential{StorageType: kindHash, StorageAlgo: p.Algorithm, Value: p.Value}
	cred.StorageAlgoParams.Salt = p.Parameters.Salt
	cred.StorageAlgoParams.Iterations = p.Parameters.Iterations
	cred.StorageAlgoParams.KeySize = p.Parameters.KeySize
	cred.StorageAlgoParams.Memory = p.Parameters.Memory
	cred.StorageAlgoParams.Parallelism = p.Parameters.Parallelism

	encoded, err := json.Marshal([]storedCredential{cred})
	if err != nil {
		return "", fmt.Errorf("failed to encode the credential for %s: %w", p.Name, err)
	}
	return string(encoded), nil
}

// hash returns the structured hash of a hash backed secret.
func (p *providerSecret) hash() (Hash, bool) {
	if p.Kind != kindHash {
		return Hash{}, false
	}
	return Hash{
		Algorithm:   p.Algorithm,
		Value:       p.Value,
		Salt:        p.Parameters.Salt,
		Iterations:  p.Parameters.Iterations,
		KeySize:     p.Parameters.KeySize,
		Memory:      p.Parameters.Memory,
		Parallelism: p.Parameters.Parallelism,
	}, true
}

// LoadAll replaces the cache with every secret the provider holds. It is called at startup so that
// resolving a reference during a request needs no outbound call.
func (r *Resolver) LoadAll(ctx context.Context) error {
	if !r.Enabled() {
		return nil
	}

	// A local store is read per name, not preloaded: it is already in memory, and holding a second
	// copy here is what would let a regenerated credential go stale.
	if r.cfg.Local != nil {
		return nil
	}

	var body struct {
		Secrets map[string]providerSecret `json:"secrets"`
	}
	if err := r.get(ctx, "/secrets", &body); err != nil {
		return err
	}

	resolved := make(map[string]string, len(body.Secrets))
	hashes := map[string]Hash{}
	for name, secret := range body.Secrets {
		secret.Name = name
		value, err := secret.substitution()
		if err != nil {
			return err
		}
		resolved[name] = value
		if h, ok := secret.hash(); ok {
			hashes[name] = h
		}
	}

	r.mu.Lock()
	r.secrets = resolved
	r.hashes = hashes
	r.lastMiss = map[string]time.Time{}
	r.mu.Unlock()
	return nil
}

// Hash describes a secret held as a one-way hash, with everything needed to verify against it.
type Hash struct {
	Algorithm   string
	Value       string
	Salt        string
	Iterations  int
	KeySize     int
	Memory      int
	Parallelism int
}

// ResolveHash returns the hash a reference points at, for verifying a presented credential.
//
// This is the counterpart to Resolve: a credential promoted from a control plane is stored as a
// reference rather than a hash, so the value to compare against lives here rather than in the
// database. The hash is returned with the parameters that produced it, because those come from
// wherever the credential was created and need not match this server's own hashing configuration.
func (r *Resolver) ResolveHash(ctx context.Context, reference string) (Hash, bool, error) {
	if !IsReference(reference) {
		return Hash{}, false, nil
	}
	if !r.Enabled() {
		return Hash{}, false, fmt.Errorf("%w: %s", ErrNotConfigured, reference)
	}

	name := ReferenceName(reference)
	if r.cfg.Local != nil {
		local, found, err := r.cfg.Local(ctx, name)
		if err != nil || !found {
			return Hash{}, false, err
		}
		secret := local.providerSecret(name)
		hash, ok := secret.hash()
		return hash, ok, nil
	}

	r.mu.RLock()
	cached, ok := r.hashes[name]
	r.mu.RUnlock()
	if ok {
		return cached, true, nil
	}

	// Not held yet: the secret may have been added after startup, so ask for it directly.
	if _, err := r.fetch(ctx, name); err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return Hash{}, false, nil
		}
		return Hash{}, false, err
	}
	r.mu.RLock()
	cached, ok = r.hashes[name]
	r.mu.RUnlock()
	return cached, ok, nil
}

// Count reports how many secrets are cached. Intended for startup logging, which must not log values.
// A resolver reading a local store caches nothing, so this is zero for one; the store reports its own
// count instead.
func (r *Resolver) Count() int {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.secrets)
}

// Resolve returns value with a secret reference replaced by its value. A value that is not a reference
// is returned unchanged, so callers can pass any stored configuration value through.
func (r *Resolver) Resolve(ctx context.Context, value string) (string, error) {
	if !IsReference(value) {
		return value, nil
	}
	if !r.Enabled() {
		return "", fmt.Errorf("%w: %s", ErrNotConfigured, value)
	}

	name := ReferenceName(value)
	if r.cfg.Local != nil {
		return r.resolveLocal(ctx, name)
	}

	r.mu.RLock()
	secret, ok := r.secrets[name]
	missedAt, missed := r.lastMiss[name]
	r.mu.RUnlock()
	if ok {
		return secret, nil
	}
	// A recent miss is not retried: the provider has already said it does not hold this name.
	if missed && time.Since(missedAt) < minRefreshInterval {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}

	secret, err := r.fetch(ctx, name)
	if err != nil {
		return "", err
	}
	return secret, nil
}

// resolveLocal reads a secret from the store held in this process. Nothing is cached: the store is
// already in memory, and a copy here would outlive a credential being regenerated.
func (r *Resolver) resolveLocal(ctx context.Context, name string) (string, error) {
	local, found, err := r.cfg.Local(ctx, name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	secret := local.providerSecret(name)
	return secret.substitution()
}

// fetch reads a single secret from the provider and caches the outcome.
func (r *Resolver) fetch(ctx context.Context, name string) (string, error) {
	var body providerSecret
	err := r.get(ctx, "/secrets/"+name, &body)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			r.mu.Lock()
			r.lastMiss[name] = time.Now()
			r.mu.Unlock()
		}
		return "", err
	}

	body.Name = name
	value, err := body.substitution()
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.secrets[name] = value
	if h, ok := body.hash(); ok {
		r.hashes[name] = h
	}
	delete(r.lastMiss, name)
	r.mu.Unlock()
	return value, nil
}

func (r *Resolver) get(ctx context.Context, path string, out interface{}) error {
	url := strings.TrimRight(r.cfg.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build secret provider request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if r.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		return fmt.Errorf("secret provider request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return ErrSecretNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("secret provider returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read secret provider response: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("failed to decode secret provider response: %w", err)
	}
	return nil
}
