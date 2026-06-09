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

// Package usage provides a registry that aggregates resource usage information
// across services. A service that owns a resource asks the registry which other
// resources reference it; the registry fans out to all registered providers.
package usage

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "UsageRegistry"

// BehaviorFallback indicates the referencing resource keeps its reference and
// resolves it to the system default at read time when the target is deleted.
const BehaviorFallback = "fallback"

// Resource type identifiers shared across usage providers and consumers.
const (
	ResourceTypeTheme       = "theme"
	ResourceTypeApplication = "application"
	ResourceTypeAgent       = "agent"
)

// ResourceUsage describes one resource that references another resource.
type ResourceUsage struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// UsagesResponse is the aggregated, serialisable result of a usage lookup.
// TotalResults and Summary are nil when usage data is unavailable (i.e. a
// provider failed to report); callers must treat nil as "unknown" and an empty
// result as "confirmed empty".
type UsagesResponse struct {
	TotalResults *int            `json:"totalResults"`
	Count        int             `json:"count"`
	Summary      map[string]int  `json:"summary"`
	Usages       []ResourceUsage `json:"usages"`
}

// Provider is the common method implemented by every service that may hold
// references to other resources. It returns the resources owned by this
// provider that reference the resource identified by (resourceType, id).
type Provider interface {
	GetResourceUsages(ctx context.Context, resourceType, id string) ([]ResourceUsage, error)
}

// Registry fans out usage lookups to all registered providers.
type Registry interface {
	RegisterProvider(p Provider)
	GetUsages(ctx context.Context, resourceType, id string) (*UsagesResponse, error)
}

// registry is the default implementation of Registry.
type registry struct {
	providers []Provider
	logger    *log.Logger
}

// NewRegistry creates a new usage registry.
func NewRegistry() Registry {
	return &registry{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// RegisterProvider adds a provider to the registry.
func (r *registry) RegisterProvider(p Provider) {
	r.providers = append(r.providers, p)
}

// GetUsages queries every registered provider for resources that reference
// (resourceType, id) and aggregates the results. If any provider fails, the
// response reports usage as unknown (nil TotalResults and Summary).
func (r *registry) GetUsages(
	ctx context.Context, resourceType, id string) (*UsagesResponse, error) {
	usages := make([]ResourceUsage, 0)
	summary := make(map[string]int)

	for _, p := range r.providers {
		providerUsages, err := p.GetResourceUsages(ctx, resourceType, id)
		if err != nil {
			r.logger.Error(ctx, "Failed to get usages from provider",
				log.String("resourceType", resourceType), log.String("id", id), log.Error(err))
			return &UsagesResponse{
				TotalResults: nil,
				Count:        0,
				Summary:      nil,
				Usages:       []ResourceUsage{},
			}, nil
		}

		usages = append(usages, providerUsages...)
		for _, u := range providerUsages {
			summary[u.ResourceType]++
		}
	}

	total := len(usages)
	return &UsagesResponse{
		TotalResults: &total,
		Count:        total,
		Summary:      summary,
		Usages:       usages,
	}, nil
}
