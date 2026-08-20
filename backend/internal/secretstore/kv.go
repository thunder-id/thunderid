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
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// KVConfig describes a key vault to keep secrets in.
//
// The fields are the ones every vault needs; anything specific to one implementation belongs to that
// implementation rather than here.
type KVConfig struct {
	// Type names the vault implementation, as registered with RegisterKV.
	Type string
	// Address is the vault's base URL.
	Address string
	// Mount is the secrets engine's mount path within the vault.
	Mount string
	// PathPrefix scopes this deployment's secrets within the mount, so deployments sharing one vault
	// do not collide.
	PathPrefix string
	// Namespace selects a vault namespace, for the implementations that have them.
	Namespace string
	// Token authenticates to the vault.
	Token string
	// CAFile is the certificate authority that signed the vault's certificate.
	CAFile string
	// Timeout bounds a single call to the vault.
	Timeout time.Duration
	// CacheTTL is how long a secret read from the vault is reused before being read again.
	CacheTTL time.Duration
}

// KVFactory builds a backend for one kind of key vault.
type KVFactory func(cfg KVConfig) (Backend, error)

var (
	kvMu        sync.RWMutex
	kvFactories = map[string]KVFactory{}
)

// RegisterKV registers a key vault implementation under a type name. It is called from an
// implementation's init, so adding a vault is a matter of adding its file rather than editing this
// one. Registering a name twice panics, which catches the mistake at startup rather than leaving
// whichever registration ran last in charge.
func RegisterKV(kind string, factory KVFactory) {
	name := strings.ToLower(strings.TrimSpace(kind))
	if name == "" {
		panic("secretstore: a key vault must be registered under a name")
	}
	kvMu.Lock()
	defer kvMu.Unlock()
	if _, exists := kvFactories[name]; exists {
		panic(fmt.Sprintf("secretstore: key vault %q is already registered", name))
	}
	kvFactories[name] = factory
}

// RegisteredKVTypes lists the key vaults that have an implementation, in order.
func RegisteredKVTypes() []string {
	kvMu.RLock()
	defer kvMu.RUnlock()
	names := make([]string, 0, len(kvFactories))
	for name := range kvFactories {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// EnvKVToken names the environment variable that supplies the key vault token.
//
// Only the token is read from the environment. Everything else about a vault (its address, mount,
// prefix, namespace) describes where the deployment points and belongs in deployment.yaml, where it
// can be read and reviewed; the token is the one value that must not be written there.
//
// It takes precedence over a configured token, because this is how a container is handed its
// credential at startup: a token that an image happened to ship must not win over the one the
// deployment actually injected.
const EnvKVToken = "THUNDERID_KV_TOKEN" // #nosec G101 -- a variable name, not a credential

// NewKVBackend builds the backend for a configured key vault, taking the token from the environment
// when one is set there.
//
// A vault with no implementation is an error rather than a fallback. Falling back would leave a
// deployment that asked for a vault running against something else, and a secret written to the wrong
// place is worse than a server that refuses to start.
func NewKVBackend(cfg KVConfig) (Backend, error) {
	kind := strings.ToLower(strings.TrimSpace(cfg.Type))
	if kind == "" {
		return nil, fmt.Errorf("secret store mode %q requires kv.type, one of %s",
			ModeKV, strings.Join(RegisteredKVTypes(), ", "))
	}

	kvMu.RLock()
	factory, ok := kvFactories[kind]
	kvMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("key vault %q is not supported, expected one of %s",
			cfg.Type, strings.Join(RegisteredKVTypes(), ", "))
	}

	if token := strings.TrimSpace(os.Getenv(EnvKVToken)); token != "" {
		cfg.Token = token
	}
	return factory(cfg)
}
