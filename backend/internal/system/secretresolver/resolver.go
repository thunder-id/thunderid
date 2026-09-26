// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package secretresolver turns a reference such as "sec:MY_APP_CLIENT_SECRET" or "var:DB_HOST" into
// its value, so
// that promoted configuration can carry the reference and leave the secret behind.
//
// A store in this process is read per reference. A provider reached over HTTP is cached, loaded at
// startup and refreshed for one name on a miss, because every miss there costs an outbound call.
package secretresolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The prefixes that mark a configuration value as a reference rather than the value itself. They
// name the collection the value is held in, which is what tells the two apart: the same name may
// exist in both, so a reference that did not say which it meant would be ambiguous.
const (
	// PrefixSecret refers to a credential, whose value is never read back by a listing.
	PrefixSecret = "sec:"
	// PrefixVariable refers to an ordinary value, which is read back as it is.
	PrefixVariable = "var:"
)

// Collection is one of the two sets of named values a provider holds. Its value is the path segment
// the provider serves it under.
type Collection string

const (
	// CollectionSecret holds credentials.
	CollectionSecret Collection = "secrets"
	// CollectionVariable holds ordinary values.
	CollectionVariable Collection = "variables"
)

// minRefreshInterval throttles single-name refreshes, so a reference that the provider does not hold
// cannot turn every request that reads it into an outbound call.
const minRefreshInterval = 30 * time.Second

// ErrNotConfigured is returned when a reference is met but no secret provider is configured.
var ErrNotConfigured = errors.New("a reference was found but no secret provider is configured")

// ErrSecretNotFound is returned when the provider does not hold the referenced secret.
var ErrSecretNotFound = errors.New("the referenced value is not available from the secret provider")

// IsReference reports whether a stored configuration value is a reference to either collection.
func IsReference(value string) bool {
	_, _, ok := parseReference(value)
	return ok
}

// ReferenceName returns the name a reference points at, without its prefix.
func ReferenceName(value string) string {
	_, name, _ := parseReference(value)
	return name
}

// ReferenceCollection returns the collection a reference names, and whether it is a reference.
func ReferenceCollection(value string) (Collection, bool) {
	coll, _, ok := parseReference(value)
	return coll, ok
}

// parseReference splits a reference into the collection it names and the name within it.
func parseReference(value string) (Collection, string, bool) {
	switch {
	case strings.HasPrefix(value, PrefixSecret):
		return CollectionSecret, strings.TrimPrefix(value, PrefixSecret), true
	case strings.HasPrefix(value, PrefixVariable):
		return CollectionVariable, strings.TrimPrefix(value, PrefixVariable), true
	default:
		return "", value, false
	}
}

// ref identifies one held value. The collection is part of the key because the same name may exist
// in both, and a cache keyed by name alone would return one where the other was asked for.
type ref struct {
	collection Collection
	name       string
}

// Config describes the secret provider to resolve against.
type Config struct {
	// BaseURL is the provider's base URL. Empty disables resolution.
	BaseURL string
	Token   string
	Timeout time.Duration
	// Local reads one secret from a store held in this process, and is used in place of BaseURL when
	// set: reading its own store over HTTP would mean this server authenticating to itself.
	//
	// It is consulted per resolution rather than cached, because the store it reads is already a
	// cache; a second copy would keep rejecting a credential until this process restarted.
	Local func(ctx context.Context, collection Collection, name string) (LocalSecret, bool, error)
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

// Resolver resolves references against a provider, caching what it loads.
type Resolver struct {
	cfg  Config
	http *http.Client

	mu sync.RWMutex
	// values holds what has been resolved, from either collection.
	values map[ref]string
	// hashes holds the structured form of every hash backed secret, for verifying a presented
	// credential against it. Only secrets appear here; a variable is not a credential.
	hashes map[ref]Hash
	// lastMiss records when a name was last fetched and not found, to throttle repeat lookups.
	lastMiss map[ref]time.Time
}

// New builds a Resolver. It performs no I/O: call LoadAll to populate the cache.
func New(cfg Config) *Resolver {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Resolver{
		cfg: cfg,
		http: &http.Client{
			Timeout:       timeout,
			CheckRedirect: redirectGuard(providerOrigin(cfg.BaseURL)),
		},
		values:   map[ref]string{},
		hashes:   map[ref]Hash{},
		lastMiss: map[ref]time.Time{},
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

	resolved := map[ref]string{}
	hashes := map[ref]Hash{}
	for _, collection := range []Collection{CollectionSecret, CollectionVariable} {
		if err := r.loadCollection(ctx, collection, resolved, hashes); err != nil {
			return err
		}
	}

	r.mu.Lock()
	r.values = resolved
	r.hashes = hashes
	r.lastMiss = map[ref]time.Time{}
	r.mu.Unlock()
	return nil
}

// loadCollection preloads one collection into the maps the caller is building.
//
// A provider that serves no variables at all answers the collection with a not-found rather than an
// empty body, and that is not a failure to start: a deployment holding only secrets is a normal
// deployment.
func (r *Resolver) loadCollection(ctx context.Context, collection Collection,
	resolved map[ref]string, hashes map[ref]Hash) error {
	var body struct {
		Secrets   map[string]providerSecret `json:"secrets"`
		Variables map[string]providerSecret `json:"variables"`
	}
	if err := r.get(ctx, "/"+string(collection), &body); err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return nil
		}
		return err
	}

	held := body.Secrets
	if collection == CollectionVariable {
		held = body.Variables
	}
	for name, value := range held {
		value.Name = name
		substituted, err := value.substitution()
		if err != nil {
			return err
		}
		key := ref{collection: collection, name: name}
		resolved[key] = substituted
		if h, ok := value.hash(); ok && collection == CollectionSecret {
			hashes[key] = h
		}
	}
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

	collection, name, _ := parseReference(reference)
	// Only a secret can back a credential. A variable is read back in the clear by anything allowed
	// to list it, so accepting one here would verify a credential against a value that is not
	// protected as one. The caller rejects on an error, which is the right way for this to fail.
	if collection != CollectionSecret {
		return Hash{}, false, fmt.Errorf(
			"a %s reference cannot back a credential: %s", PrefixVariable, reference)
	}

	key := ref{collection: collection, name: name}
	if r.cfg.Local != nil {
		local, found, err := r.cfg.Local(ctx, collection, name)
		if err != nil || !found {
			return Hash{}, false, err
		}
		secret := local.providerSecret(name)
		hash, ok := secret.hash()
		return hash, ok, nil
	}

	r.mu.RLock()
	cached, ok := r.hashes[key]
	missedAt, missed := r.lastMiss[key]
	r.mu.RUnlock()
	if ok {
		return cached, true, nil
	}
	// A recent miss is not retried, the same as Resolve. This runs on every authentication against a
	// credential held as a reference, so without it a name the provider does not hold would mean an
	// outbound request per attempt.
	if missed && time.Since(missedAt) < minRefreshInterval {
		return Hash{}, false, nil
	}

	// Not held yet: the secret may have been added after startup, so ask for it directly.
	if _, err := r.fetch(ctx, key); err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			return Hash{}, false, nil
		}
		return Hash{}, false, err
	}
	r.mu.RLock()
	cached, ok = r.hashes[key]
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
	return len(r.values)
}

// Resolve returns value with a secret reference replaced by its value. A value that is not a reference
// is returned unchanged, so callers can pass any stored configuration value through.
func (r *Resolver) Resolve(ctx context.Context, value string) (string, error) {
	collection, name, isRef := parseReference(value)
	if !isRef {
		return value, nil
	}
	if !r.Enabled() {
		return "", fmt.Errorf("%w: %s", ErrNotConfigured, value)
	}

	key := ref{collection: collection, name: name}
	if r.cfg.Local != nil {
		return r.resolveLocal(ctx, key)
	}

	r.mu.RLock()
	held, ok := r.values[key]
	missedAt, missed := r.lastMiss[key]
	r.mu.RUnlock()
	if ok {
		return held, nil
	}
	// A recent miss is not retried: the provider has already said it does not hold this name.
	if missed && time.Since(missedAt) < minRefreshInterval {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, value)
	}

	held, err := r.fetch(ctx, key)
	if err != nil {
		return "", err
	}
	return held, nil
}

// resolveLocal reads a secret from the store held in this process. Nothing is cached: the store is
// already in memory, and a copy here would outlive a credential being regenerated.
func (r *Resolver) resolveLocal(ctx context.Context, key ref) (string, error) {
	local, found, err := r.cfg.Local(ctx, key.collection, key.name)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, key.name)
	}
	held := local.providerSecret(key.name)
	return held.substitution()
}

// fetch reads a single secret from the provider and caches the outcome.
func (r *Resolver) fetch(ctx context.Context, key ref) (string, error) {
	segment, err := secretPathSegment(key.name)
	if err != nil {
		return "", err
	}

	var body providerSecret
	err = r.get(ctx, "/"+string(key.collection)+"/"+segment, &body)
	if err != nil {
		if errors.Is(err, ErrSecretNotFound) {
			r.mu.Lock()
			r.lastMiss[key] = time.Now()
			r.mu.Unlock()
		}
		return "", err
	}

	body.Name = key.name
	value, err := body.substitution()
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	r.values[key] = value
	// Only a secret can be held as a hash. A variable carrying hash material would be a provider
	// answering the wrong shape, and recording it would let a variable back a credential.
	if h, ok := body.hash(); ok && key.collection == CollectionSecret {
		r.hashes[key] = h
	}
	delete(r.lastMiss, key)
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

	// The response carries the secret, so the connection has to protect it whether or not a token
	// is configured. Checking this only when sending a token would leave a provider reached without
	// one returning credentials in the clear. A provider on this host is the exception: nothing
	// leaves the machine, which is what a local development setup runs.
	if !carriesSecretsSafely(req.URL) {
		return fmt.Errorf(
			"refusing to read a secret over %s from %s: configure an https provider URL",
			req.URL.Scheme, req.URL.Host)
	}
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

// schemeHTTPS is the only scheme the provider token is sent over.
const schemeHTTPS = "https"

// secretPathSegment escapes a secret name for use as one path segment.
//
// A name reaches this from stored configuration, so it is not necessarily well formed. Without
// escaping, one carrying a slash, a query character or a traversal sequence would address something
// other than the secret it names.
func secretPathSegment(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: a secret name is required", ErrSecretNotFound)
	}
	escaped := url.PathEscape(trimmed)
	if escaped == "." || escaped == ".." {
		return "", fmt.Errorf("%w: %s", ErrSecretNotFound, name)
	}
	return escaped, nil
}

// carriesSecretsSafely reports whether a secret may cross this address: over TLS, or to a provider
// on this host, where the request never reaches a network. It governs the request and the response
// alike, since the token goes one way and the secret comes back the other.
func carriesSecretsSafely(u *url.URL) bool {
	if u.Scheme == schemeHTTPS {
		return true
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// redirectGuard stops a redirect carrying the token or the secret somewhere it should not go.
//
// Two things are wrong with following one freely. Go drops the Authorization header when the host
// changes, but keeps it for the same host over plain http and for a subdomain of it, so a provider
// that redirects to a subdomain it does not control hands the bearer token to whoever does. And the
// response to a redirect carries the secret, so the hop has to be protected however it is reached.
//
// Both are closed by confining a redirect to the configured origin: the provider answers at the
// address it was configured at, or the read fails.
func redirectGuard(origin string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if !carriesSecretsSafely(req.URL) {
			return fmt.Errorf("refusing to follow a secret provider redirect to %s://%s",
				req.URL.Scheme, req.URL.Host)
		}
		if origin != "" && req.URL.Host != origin {
			return fmt.Errorf("refusing to follow a secret provider redirect away from %s to %s",
				origin, req.URL.Host)
		}
		return nil
	}
}

// providerOrigin returns the host a configured provider answers at, empty when there is none to
// parse. A redirect is measured against it.
func providerOrigin(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return ""
	}
	return parsed.Host
}
