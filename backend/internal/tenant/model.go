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

// Package tenant provides the Control Plane platform ("system" tenant) APIs for provisioning,
// listing, and deprovisioning other tenants. These APIs are usable only by the reserved system
// tenant; a regular tenant's token cannot reach them.
package tenant

// Tenant is the API representation of a managed tenant recorded in the platform registry.
type Tenant struct {
	ID           string `json:"id,omitempty"`
	DeploymentID string `json:"deploymentId"`
	Name         string `json:"name,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// TenantListResponse is the response for listing managed tenants.
type TenantListResponse struct {
	TotalResults int      `json:"totalResults"`
	Count        int      `json:"count"`
	Tenants      []Tenant `json:"tenants"`
}

// CreateTenantRequest is the request body for provisioning a new tenant.
type CreateTenantRequest struct {
	// Org is the organization, and is the deployment id: the organization has one workspace. Env names
	// the first environment to register against it, which is a resource of that workspace rather than
	// a deployment of its own.
	Org string `json:"org" native:"required,min=1,max=120"`
	Env string `json:"env" native:"required,min=1,max=120"`

	Name string `json:"name,omitempty" native:"max=255"`
	// Rank orders the environment in its organization's promotion chain: configuration moves from a
	// lower rank to the next one up. The first environment of an organization is always rank 1,
	// whatever is asked for, because there is nothing below it. Omitted on a later environment, it
	// goes to the end of the chain.
	Rank *int `json:"rank,omitempty"`
	// ControlPlane describes reaching this server, which the environment reads its configuration from
	// when a version is captured. Only the parts this server cannot know for itself are settable.
	ControlPlane *ControlPlane `json:"controlPlane,omitempty"`
	// DataPlane is the deployment this environment's configuration is applied to. With one given, the
	// environment is registered for promotion as the tenant is created; without one there is nowhere
	// to apply to, so only the tenant is created and the environment is registered later.
	DataPlane *DataPlane `json:"dataPlane,omitempty"`
}

// ControlPlane is how an environment reaches the control plane it reads from.
type ControlPlane struct {
	// InsecureSkipVerify skips TLS verification when capturing a version. A control plane serving a
	// certificate its own clients do not trust, which is every local deployment, is unreachable
	// without it.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// DataPlane is the deployment an environment's configuration is applied to.
//
// It is named rather than addressed. The data plane dials this control plane and holds that
// connection open, and everything sent to it travels back down that connection, so there is no URL to
// call and no credential to hold. ID is what the data plane presents when it connects.
type DataPlane struct {
	ID string `json:"id"`
	// BaseURL is where that deployment serves its own users. Nothing calls it; it is recorded so an
	// operator can follow it, and so the environment's Console can be pointed at it.
	BaseURL string `json:"baseUrl,omitempty"`
}

// RegisterEnvironmentRequest registers an existing tenant as an environment of its organization.
//
// A tenant created without a data plane has no environment: there was nowhere to apply to. This is
// how one is registered once that data plane exists, without needing a token for the tenant itself.
type RegisterEnvironmentRequest struct {
	// Env names the environment within its organization, e.g. "stage". It is given here rather than
	// read from the deployment id because the deployment is the organization: one workspace holding
	// every environment, none of which the id names.
	Env string `json:"env" native:"required,min=1,max=120"`
	// DataPlane is the deployment this environment's configuration is applied to.
	DataPlane DataPlane `json:"dataPlane" native:"required"`
	// Rank orders the environment in its organization's promotion chain. The first environment of an
	// organization is always rank 1, whatever is asked for. Omitted on a later one, it goes to the end.
	Rank *int `json:"rank,omitempty"`
	// ControlPlane describes reaching this server, which the environment reads its configuration from.
	ControlPlane *ControlPlane `json:"controlPlane,omitempty"`
}

// CreateTenantResponse is the created tenant, and for an environment seeded from an existing one, what
// that copy did.
type CreateTenantResponse struct {
	Tenant
	// Seeded is absent for the first environment of an organization, which is provisioned from the
	// bootstrap baseline rather than copied.
	// Environment is the promotion entry registered for this tenant, absent when no data plane was
	// given to apply to.
	Environment *EnvironmentSummary `json:"environment,omitempty"`
}

// EnvironmentSummary is the promotion entry a new tenant was registered as.
type EnvironmentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Rank int    `json:"rank"`
	// DataPlaneToken is the credential this environment's Data Plane connects with, returned here and
	// nowhere else. Mount it on that deployment; it cannot be read back afterwards, only reissued.
	DataPlaneToken string `json:"dataPlaneToken,omitempty"`
}
