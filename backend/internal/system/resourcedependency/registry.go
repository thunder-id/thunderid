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

// Package resourcedependency provides a registry that aggregates resource dependency
// information across services. A service that owns a resource asks the registry which
// other resources reference it; the registry fans out to all registered providers.
package resourcedependency

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const loggerComponentName = "DependencyRegistry"

// BehaviorFallback indicates the referencing resource keeps its reference and
// resolves it to the system default at read time when the target is deleted.
const BehaviorFallback = "fallback"

// BehaviorRestrict indicates the referencing resource forbids deletion of the target while the
// reference exists; the caller must remove or reassign the reference before the target can be deleted.
const BehaviorRestrict = "restrict"

// isBlocking reports whether a behaviorOnDelete value forbids deletion of the referenced resource.
func isBlocking(behaviorOnDelete string) bool {
	return behaviorOnDelete == BehaviorRestrict
}

// BlockingUsages returns the dependencies in the response whose behaviorOnDelete forbids deletion
// of the target resource. Returns an empty slice when there are none.
func BlockingUsages(resp *DependenciesResponse) []ResourceDependency {
	blocking := make([]ResourceDependency, 0)
	if resp == nil {
		return blocking
	}
	for _, u := range resp.Usages {
		if isBlocking(u.BehaviorOnDelete) {
			blocking = append(blocking, u)
		}
	}
	return blocking
}

// PaginateUsages narrows resp.Usages to the [offset, offset+limit) window in place and updates
// Count to reflect the returned page. TotalResults and Summary continue to describe the full result
// set. A nil response, or one whose TotalResults is nil (usage unavailable), is returned unchanged.
func PaginateUsages(resp *DependenciesResponse, limit, offset int) *DependenciesResponse {
	if resp == nil || resp.TotalResults == nil {
		return resp
	}

	total := len(resp.Usages)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	resp.Usages = resp.Usages[start:end]
	resp.Count = len(resp.Usages)
	return resp
}

// Resource type identifiers shared across dependency providers and consumers.
const (
	ResourceTypeTheme              = "theme"
	ResourceTypeLayout             = "layout"
	ResourceTypeFlow               = "flow"
	ResourceTypeUser               = "user"
	ResourceTypeApplication        = "application"
	ResourceTypeAgent              = "agent"
	ResourceTypeGroup              = "group"
	ResourceTypeIDP                = "idp"
	ResourceTypeNotificationSender = "notificationSender"
	ResourceTypeOU                 = "organizationUnit"
	ResourceTypeResourceServer     = "resourceServer"
	ResourceTypeResource           = "resource"
)

// SummarizeBlockingUsages renders a deterministic, human-readable summary of blocking dependencies
// grouped by resource type, e.g. "2 flow(s)".
func SummarizeBlockingUsages(usages []ResourceDependency) string {
	counts := make(map[string]int)
	for _, u := range usages {
		counts[u.ResourceType]++
	}
	types := make([]string, 0, len(counts))
	for rt := range counts {
		types = append(types, rt)
	}
	sort.Strings(types)
	parts := make([]string, 0, len(types))
	for _, rt := range types {
		parts = append(parts, fmt.Sprintf("%d %s(s)", counts[rt], rt))
	}
	return strings.Join(parts, ", ")
}

// ResourceDependency describes one resource that references another resource.
type ResourceDependency struct {
	ResourceType     string `json:"resourceType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
	BehaviorOnDelete string `json:"behaviorOnDelete"`
}

// DependenciesResponse is the aggregated, serialisable result of a dependency lookup.
// TotalResults and Summary are nil when dependency data is unavailable (i.e. a
// provider failed to report); callers must treat nil as "unknown" and an empty
// result as "confirmed empty".
type DependenciesResponse struct {
	TotalResults *int                 `json:"totalResults"`
	Count        int                  `json:"count"`
	Summary      map[string]int       `json:"summary"`
	Usages       []ResourceDependency `json:"usages"`
}

// Provider is the common method implemented by every service that may hold
// references to other resources. It returns the resources owned by this
// provider that reference the resource identified by (resourceType, id).
type Provider interface {
	GetResourceDependencies(ctx context.Context, resourceType, id string) ([]ResourceDependency, error)
}

// CascadeDeleter is an optional interface a Provider may implement to delete its cascade-behavior
// dependents of a target resource when the target is deleted. Implementations must be idempotent
// and return the number of dependents removed.
type CascadeDeleter interface {
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)
}

// Registry fans out dependency lookups to all registered providers.
type Registry interface {
	RegisterProvider(p Provider)
	GetDependencies(ctx context.Context, resourceType, id string) (*DependenciesResponse, error)
	CascadeDelete(ctx context.Context, resourceType, id string) (int, error)
}

// registry is the default implementation of Registry.
type registry struct {
	providers []Provider
	logger    *log.Logger
}

// Initialize creates a dependency registry and registers the given providers.
func Initialize(providers ...Provider) Registry {
	r := newRegistry()
	for _, p := range providers {
		r.RegisterProvider(p)
	}
	return r
}

// newRegistry creates a new dependency registry.
func newRegistry() Registry {
	return &registry{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// RegisterProvider adds a provider to the registry. A nil provider — e.g. a service that
// failed to initialize and was wired in regardless — is ignored so it cannot panic a later
// usage lookup.
func (r *registry) RegisterProvider(p Provider) {
	if p == nil {
		r.logger.Warn(context.Background(), "Ignoring nil usage provider registration")
		return
	}
	r.providers = append(r.providers, p)
}

// GetDependencies queries every registered provider for resources that reference
// (resourceType, id) and aggregates the results. If any provider fails, the
// response reports the result as unknown (nil TotalResults and Summary).
func (r *registry) GetDependencies(
	ctx context.Context, resourceType, id string) (*DependenciesResponse, error) {
	usages := make([]ResourceDependency, 0)
	summary := make(map[string]int)

	for _, p := range r.providers {
		providerUsages, err := p.GetResourceDependencies(ctx, resourceType, id)
		if err != nil {
			r.logger.Error(ctx, "Failed to get dependencies from provider",
				log.String("resourceType", resourceType), log.String("id", id), log.Error(err))
			return &DependenciesResponse{
				TotalResults: nil,
				Count:        0,
				Summary:      nil,
				Usages:       []ResourceDependency{},
			}, nil
		}

		usages = append(usages, providerUsages...)
		for _, u := range providerUsages {
			summary[u.ResourceType]++
		}
	}

	total := len(usages)
	return &DependenciesResponse{
		TotalResults: &total,
		Count:        total,
		Summary:      summary,
		Usages:       usages,
	}, nil
}

// CascadeDelete asks every provider that implements CascadeDeleter to remove its cascade-behavior
// dependents of (resourceType, id), returning the total number removed. It stops and returns the
// error on the first provider failure, so the caller can abort the target deletion.
func (r *registry) CascadeDelete(ctx context.Context, resourceType, id string) (int, error) {
	total := 0
	for _, p := range r.providers {
		cascader, ok := p.(CascadeDeleter)
		if !ok {
			continue
		}
		deleted, err := cascader.CascadeDeleteDependencies(ctx, resourceType, id)
		if err != nil {
			r.logger.Error(ctx, "Failed to cascade-delete dependencies from provider",
				log.String("resourceType", resourceType), log.String("id", id), log.Error(err))
			return total, err
		}
		total += deleted
	}
	return total, nil
}
