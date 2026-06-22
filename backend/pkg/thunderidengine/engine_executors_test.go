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

package thunderidengine

import (
	"fmt"
	"testing"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
)

// fakeRegistry is an in-memory ExecutorRegistry used to assert how custom executors are layered
// onto the engine's registry without standing up the full engine.
type fakeRegistry struct {
	m map[string]ExecutorInterface
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{m: make(map[string]ExecutorInterface)}
}

func (r *fakeRegistry) GetExecutor(name string) (ExecutorInterface, error) {
	ex, ok := r.m[name]
	if !ok {
		return nil, fmt.Errorf("executor %q not registered", name)
	}
	return ex, nil
}

func (r *fakeRegistry) RegisterExecutor(name string, ex ExecutorInterface) { r.m[name] = ex }

func (r *fakeRegistry) IsRegistered(name string) bool {
	_, ok := r.m[name]
	return ok
}

// stubExecutor is a minimal custom executor. It embeds a base built via NewBaseExecutor so it
// inherits the boilerplate ExecutorInterface methods, exercising the public authoring surface.
type stubExecutor struct {
	ExecutorInterface
}

func newStubExecutor(name string) ExecutorInterface {
	return &stubExecutor{
		ExecutorInterface: NewBaseExecutor(name, flowcommon.ExecutorTypeUtility, nil, nil),
	}
}

func TestApplyCustomExecutors_RegistersAlongsideBuiltIns(t *testing.T) {
	reg := newFakeRegistry()
	reg.RegisterExecutor("CredentialsAuthExecutor", newStubExecutor("CredentialsAuthExecutor"))

	c := &engineConfig{
		executorRegistry: reg,
		customExecutors: map[string]ExecutorInterface{
			"MyCustomExecutor": newStubExecutor("MyCustomExecutor"),
		},
	}

	if err := c.applyCustomExecutors(); err != nil {
		t.Fatalf("applyCustomExecutors returned error: %v", err)
	}
	if !reg.IsRegistered("MyCustomExecutor") {
		t.Error("custom executor was not registered")
	}
	if !reg.IsRegistered("CredentialsAuthExecutor") {
		t.Error("enabled built-in executor was lost")
	}
}

func TestApplyCustomExecutors_OverridesBuiltInOnNameClash(t *testing.T) {
	reg := newFakeRegistry()
	builtIn := newStubExecutor("CredentialsAuthExecutor")
	reg.RegisterExecutor("CredentialsAuthExecutor", builtIn)

	override := newStubExecutor("CredentialsAuthExecutor")
	c := &engineConfig{
		executorRegistry: reg,
		customExecutors:  map[string]ExecutorInterface{"CredentialsAuthExecutor": override},
	}

	if err := c.applyCustomExecutors(); err != nil {
		t.Fatalf("applyCustomExecutors returned error: %v", err)
	}
	got, err := reg.GetExecutor("CredentialsAuthExecutor")
	if err != nil {
		t.Fatalf("GetExecutor returned error: %v", err)
	}
	if got != override {
		t.Error("custom executor did not override the built-in on name clash")
	}
}

func TestApplyCustomExecutors_NoneIsNoOp(t *testing.T) {
	c := &engineConfig{} // no registry, no custom executors
	if err := c.applyCustomExecutors(); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}
}

func TestApplyCustomExecutors_RequiresRegistry(t *testing.T) {
	c := &engineConfig{
		customExecutors: map[string]ExecutorInterface{"MyCustomExecutor": newStubExecutor("MyCustomExecutor")},
	}
	if err := c.applyCustomExecutors(); err == nil {
		t.Fatal("expected an error when custom executors are supplied without a registry")
	}
}

func TestApplyCustomExecutors_RejectsNilExecutor(t *testing.T) {
	c := &engineConfig{
		executorRegistry: newFakeRegistry(),
		customExecutors:  map[string]ExecutorInterface{"MyCustomExecutor": nil},
	}
	if err := c.applyCustomExecutors(); err == nil {
		t.Fatal("expected an error when a custom executor is nil")
	}
}

func TestWithCustomExecutors_MergesAcrossCalls(t *testing.T) {
	var c engineConfig
	WithCustomExecutors(map[string]ExecutorInterface{"A": newStubExecutor("A")})(&c)
	WithCustomExecutors(map[string]ExecutorInterface{"B": newStubExecutor("B")})(&c)

	if len(c.customExecutors) != 2 {
		t.Fatalf("expected 2 custom executors after two calls, got %d", len(c.customExecutors))
	}
	if c.customExecutors["A"] == nil || c.customExecutors["B"] == nil {
		t.Error("WithCustomExecutors did not merge entries across calls")
	}
}
