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

package resourcedependency

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubProvider struct {
	usages []ResourceDependency
	err    error
}

func (s *stubProvider) GetResourceDependencies(
	_ context.Context, _, _ string) ([]ResourceDependency, error) {
	return s.usages, s.err
}

func TestGetDependenciesAggregatesAcrossProviders(t *testing.T) {
	reg := newRegistry()
	reg.RegisterProvider(&stubProvider{usages: []ResourceDependency{
		{ResourceType: "application", ID: "a1", DisplayName: "App 1", BehaviorOnDelete: BehaviorFallback},
		{ResourceType: "application", ID: "a2", DisplayName: "App 2", BehaviorOnDelete: BehaviorFallback},
	}})
	reg.RegisterProvider(&stubProvider{usages: []ResourceDependency{
		{ResourceType: "agent", ID: "g1", DisplayName: "Agent 1", BehaviorOnDelete: BehaviorFallback},
	}})

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	assert.NoError(t, err)
	assert.NotNil(t, resp.TotalResults)
	assert.Equal(t, 3, *resp.TotalResults)
	assert.Equal(t, 3, resp.Count)
	assert.Len(t, resp.Usages, 3)
	assert.Equal(t, map[string]int{"application": 2, "agent": 1}, resp.Summary)
}

func TestGetDependenciesProviderErrorReturnsUnknown(t *testing.T) {
	reg := newRegistry()
	reg.RegisterProvider(&stubProvider{usages: []ResourceDependency{
		{ResourceType: "application", ID: "a1", DisplayName: "App 1", BehaviorOnDelete: BehaviorFallback},
	}})
	reg.RegisterProvider(&stubProvider{err: errors.New("lookup failed")})

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	assert.NoError(t, err)
	assert.Nil(t, resp.TotalResults)
	assert.Nil(t, resp.Summary)
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Usages)
}

func TestGetDependenciesNoProvidersReturnsEmpty(t *testing.T) {
	reg := newRegistry()

	resp, err := reg.GetDependencies(context.Background(), "theme", "t1")

	assert.NoError(t, err)
	assert.NotNil(t, resp.TotalResults)
	assert.Equal(t, 0, *resp.TotalResults)
	assert.Empty(t, resp.Usages)
	assert.Empty(t, resp.Summary)
}
