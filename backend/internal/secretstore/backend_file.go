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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// fileBackend keeps secrets in a JSON file beside the server.
//
// The file belongs to this instance alone, so a deployment running several of them does not share
// what it stores. That is why it reports no cache TTL: nothing else writes here, so what was loaded
// cannot go stale behind this process's back.
type fileBackend struct {
	path string
	// mu serializes the read-modify-write of the whole file, which is how a single entry is changed.
	mu sync.Mutex
}

// NewFileBackend builds a backend over the JSON file at path. The file is created on the first write,
// so a deployment that has never stored a secret starts with an empty store rather than an error.
func NewFileBackend(path string) (Backend, error) {
	if path == "" {
		return nil, fmt.Errorf("a file-backed secret store needs a path")
	}
	return &fileBackend{path: path}, nil
}

func (b *fileBackend) Name() string { return "file:" + b.path }

// CacheTTL is zero: this file has no writer but this process.
func (b *fileBackend) CacheTTL() time.Duration { return 0 }

// Load reads every secret from the file. A file that is not there yet is an empty store.
func (b *fileBackend) Load(context.Context) (map[string]Secret, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.read()
}

// Put stores a secret, replacing any entry of the same name.
func (b *fileBackend) Put(_ context.Context, secret Secret) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	secrets, err := b.read()
	if err != nil {
		return err
	}
	secrets[secret.Name] = secret
	return b.write(secrets)
}

// Delete removes a secret. Removing an absent name is not an error, so a repeated delete is safe.
func (b *fileBackend) Delete(_ context.Context, name string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	secrets, err := b.read()
	if err != nil {
		return err
	}
	if _, held := secrets[name]; !held {
		return nil
	}
	delete(secrets, name)
	return b.write(secrets)
}

// read parses the file. Callers hold b.mu.
func (b *fileBackend) read() (map[string]Secret, error) {
	raw, err := os.ReadFile(b.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Secret{}, nil
		}
		return nil, fmt.Errorf("failed to read secret store %s: %w", b.path, err)
	}

	secrets := map[string]Secret{}
	if err := json.Unmarshal(raw, &secrets); err != nil {
		// A plain name to value object is also accepted, so an operator can hand-write a simple file.
		var plain map[string]string
		if plainErr := json.Unmarshal(raw, &plain); plainErr != nil {
			return nil, fmt.Errorf("failed to parse secret store %s: %w", b.path, err)
		}
		secrets = map[string]Secret{}
		for name, value := range plain {
			secrets[name] = Secret{Name: name, Kind: KindValue, Value: value}
		}
	}
	// A hand-written file may leave the name out of the entry, since it is already the key.
	for name, secret := range secrets {
		secret.Name = name
		secrets[name] = secret
	}
	return secrets, nil
}

// write replaces the file. Callers hold b.mu.
func (b *fileBackend) write(secrets map[string]Secret) error {
	raw, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode secrets: %w", err)
	}

	if dir := filepath.Dir(b.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("failed to create secret store directory: %w", err)
		}
	}
	// Written to a temporary file and renamed, so a crash mid-write cannot leave a truncated store.
	tmp := b.path + ".tmp"
	// Secrets are readable by their owner only.
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("failed to write secret store: %w", err)
	}
	if err := os.Rename(tmp, b.path); err != nil {
		return fmt.Errorf("failed to commit secret store: %w", err)
	}
	return nil
}
