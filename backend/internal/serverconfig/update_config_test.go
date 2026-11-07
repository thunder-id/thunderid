// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// memStore is a writable layer per section, written conditionally as the database writes it. beforeWrite,
// when set, runs between a read and the write that follows it, as another node's write would.
type memStore struct {
	mu          sync.Mutex
	layers      map[ConfigName]json.RawMessage
	readOnly    json.RawMessage
	beforeWrite func()
	writes      int
}

func (m *memStore) GetServerConfig(_ context.Context, name ConfigName) (storeLayers, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return storeLayers{ReadOnly: m.readOnly, Writable: m.layers[name]}, nil
}

func (m *memStore) UpsertServerConfig(_ context.Context, cfg ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layers[cfg.Name] = cfg.Value
	return nil
}

func (m *memStore) ReplaceServerConfigIf(_ context.Context, cfg ServerConfig, expected json.RawMessage) (bool,
	error) {
	if hook := m.beforeWrite; hook != nil {
		m.beforeWrite = nil
		hook()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes++
	current, held := m.layers[cfg.Name]
	if (expected == nil && held) || (expected != nil && string(current) != string(expected)) {
		return false, nil
	}
	m.layers[cfg.Name] = cfg.Value
	return true, nil
}

// mapHandler decodes a section as a JSON object, refusing a declarative layer's write like a section that
// is set declaratively.
type mapHandler struct{}

func (mapHandler) Decode(raw json.RawMessage) (any, error) {
	value := map[string]any{}
	if len(raw) == 0 {
		return value, nil
	}
	err := json.Unmarshal(raw, &value)
	return value, err
}

func (mapHandler) Validate(_ context.Context, _, readOnly, _ any) error {
	if ro, _ := readOnly.(map[string]any); len(ro) > 0 {
		return errors.New("read-only")
	}
	return nil
}

func (mapHandler) Merge(_, writable any) any { return writable }

// setKey is an update that sets one key of the section.
type setKey struct{ key, value string }

func (u setKey) Update(current any) (any, error) {
	next := map[string]any{}
	for key, value := range current.(map[string]any) {
		next[key] = value
	}
	next[u.key] = u.value
	return next, nil
}

type refuse struct{}

func (refuse) Update(any) (any, error) { return nil, errors.New("no") }

func newUpdateService(store *memStore) ServerConfigService {
	return newServerConfigService(store, map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: mapHandler{}},
		nil)
}

// Updates to different keys of one section, from two nodes at once, all survive: one that loses the
// conditional write reads the section again and writes over the other's.
func TestConcurrentUpdatesToDifferentKeysBothSurvive(t *testing.T) {
	store := &memStore{layers: map[ConfigName]json.RawMessage{}}
	nodes := []ServerConfigService{newUpdateService(store), newUpdateService(store)}
	var wg sync.WaitGroup
	for i, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		wg.Add(1)
		go func(node ServerConfigService, key string) {
			defer wg.Done()
			if svcErr := node.UpdateConfig(context.Background(), ConfigNameCORS, setKey{key, "on"}); svcErr != nil {
				t.Errorf("update %s: %v", key, svcErr.Code)
			}
		}(nodes[i%2], key)
	}
	wg.Wait()

	var section map[string]any
	require.NoError(t, json.Unmarshal(store.layers[ConfigNameCORS], &section))
	assert.Len(t, section, 10, "an update was lost")
}

// A write that loses to one made after its read reads the section again and keeps that write.
func TestAnUpdateThatLosesReadsAgain(t *testing.T) {
	store := &memStore{layers: map[ConfigName]json.RawMessage{ConfigNameCORS: json.RawMessage(`{"a":"1"}`)}}
	store.beforeWrite = func() { store.layers[ConfigNameCORS] = json.RawMessage(`{"a":"1","b":"2"}`) }

	svcErr := newUpdateService(store).UpdateConfig(context.Background(), ConfigNameCORS, setKey{"c", "3"})

	require.Nil(t, svcErr)
	assert.JSONEq(t, `{"a":"1","b":"2","c":"3"}`, string(store.layers[ConfigNameCORS]))
	assert.Equal(t, 2, store.writes)
}

// A section that keeps changing is given up on, a refused update and a declarative section are invalid
// values, and an unknown section is refused.
func TestUpdateRefusals(t *testing.T) {
	ctx := context.Background()
	busy := &busyStore{memStore: memStore{layers: map[ConfigName]json.RawMessage{}}}
	svc := newServerConfigService(busy,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: mapHandler{}}, nil)
	assert.Equal(t, &ErrorConfigWriteConflict, svc.UpdateConfig(ctx, ConfigNameCORS, setKey{"a", "1"}))
	assert.Equal(t, maxConfigUpdateAttempts, busy.writes)

	store := &memStore{layers: map[ConfigName]json.RawMessage{}}
	assert.Equal(t, &ErrorInvalidConfigValue, newUpdateService(store).UpdateConfig(ctx, ConfigNameCORS, refuse{}))
	store.readOnly = json.RawMessage(`{"locked":"yes"}`)
	assert.Equal(t, &ErrorInvalidConfigValue, newUpdateService(store).UpdateConfig(ctx, ConfigNameCORS,
		setKey{"a", "1"}))
	assert.Equal(t, &ErrorUnsupportedConfigName, newUpdateService(store).UpdateConfig(ctx, "nope", setKey{"a", "1"}))
	assert.Empty(t, store.layers)

	failing := &failingStore{}
	svc = newServerConfigService(failing,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: mapHandler{}}, nil)
	assert.Equal(t, &common.InternalServerError, svc.UpdateConfig(ctx, ConfigNameCORS, setKey{"a", "1"}))
}

// busyStore loses every conditional write, as a section another node keeps writing does.
type busyStore struct{ memStore }

func (b *busyStore) ReplaceServerConfigIf(context.Context, ServerConfig, json.RawMessage) (bool, error) {
	b.writes++
	return false, nil
}

// failingStore cannot write.
type failingStore struct{ memStore }

func (f *failingStore) ReplaceServerConfigIf(context.Context, ServerConfig, json.RawMessage) (bool, error) {
	return false, errors.New("db down")
}

// An update whose request ends while it waits to try again stops waiting and gives up.
func TestAnUpdateStopsWaitingWhenItsRequestEnds(t *testing.T) {
	busy := &busyStore{memStore: memStore{layers: map[ConfigName]json.RawMessage{}}}
	svc := newServerConfigService(busy,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: mapHandler{}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Equal(t, &ErrorConfigWriteConflict, svc.UpdateConfig(ctx, ConfigNameCORS, setKey{"a", "1"}))
	assert.Equal(t, 1, busy.writes, "it tried again after its request ended")
}
