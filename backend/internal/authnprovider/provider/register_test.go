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

package provider

import (
	"testing"

	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

func TestBuiltInAuthnProviderRegistrars_CatalogContents(t *testing.T) {
	catalog := newBuiltInAuthnProviderRegistrars()
	for _, want := range []string{"default", "rest"} {
		if _, ok := catalog[want]; !ok {
			t.Errorf("expected catalog entry %q to be present", want)
		}
	}
}

func TestBuiltInAuthnProviderRegistrars_RestEnabledFlag(t *testing.T) {
	catalog := newBuiltInAuthnProviderRegistrars()
	registrar, ok := catalog["rest"]
	if !ok {
		t.Fatalf("rest registrar missing from catalog")
	}

	// Absent / empty / enabled=false properties => opt out (nil, nil), not an error.
	cases := []map[string]interface{}{
		nil,
		{},
		{"enabled": false, "base_url": "https://example.com"},
		// Staged config without an enabled flag stays inert — even with a base_url set.
		{"base_url": "https://example.com", "timeout": 5},
	}
	for i, props := range cases {
		p, err := registrar(props, AuthnProviderDependencies{})
		if err != nil || p != nil {
			t.Errorf("case %d: expected (nil, nil) when not enabled; got (%v, %v)", i, p, err)
		}
	}

	// enabled=true but base_url missing or empty => misconfiguration, must error.
	missingBaseURL := map[string]interface{}{"enabled": true, "timeout": 5}
	if _, err := registrar(missingBaseURL, AuthnProviderDependencies{}); err == nil {
		t.Errorf("expected error when enabled is true but base_url is missing")
	}
	emptyBaseURL := map[string]interface{}{"enabled": true, "base_url": ""}
	if _, err := registrar(emptyBaseURL, AuthnProviderDependencies{}); err == nil {
		t.Errorf("expected error when enabled is true but base_url is empty")
	}
	// Successful construction is exercised by the existing REST provider tests; the
	// catalog wrapper just forwards parsed properties. Re-running it here would pull
	// in the server runtime config, which isn't initialized in unit tests.
}

func TestBuiltInAuthnProviderRegistrars_DefaultRegistrar(t *testing.T) {
	catalog := newBuiltInAuthnProviderRegistrars()
	registrar, ok := catalog["default"]
	if !ok {
		t.Fatalf("default registrar missing from catalog")
	}

	deps := AuthnProviderDependencies{
		EntitySvc: entitymock.NewEntityServiceInterfaceMock(t),
	}
	p, err := registrar(nil, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatalf("expected non-nil default provider")
	}
}

func TestPropsInt(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want int
	}{
		{"int", 7, 7},
		{"int64", int64(8), 8},
		{"float64", float64(9), 9},
		{"string returns zero", "5", 0},
		{"nil returns zero", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := propsInt(tc.in); got != tc.want {
				t.Errorf("propsInt(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
