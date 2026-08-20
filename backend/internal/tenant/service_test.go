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

package tenant

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/bootstrap"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/importer"
)

const systemID = "root"

// fakeStore is an in-memory tenantStoreInterface.
type fakeStore struct {
	provisioned map[string]bool
	registry    map[string]Tenant
	purged      []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{provisioned: map[string]bool{}, registry: map[string]Tenant{}}
}

func (s *fakeStore) CreateTenant(_ context.Context, t Tenant) error {
	s.registry[t.DeploymentID] = t
	s.provisioned[t.DeploymentID] = true
	return nil
}

func (s *fakeStore) GetTenant(_ context.Context, deploymentID string) (Tenant, error) {
	t, ok := s.registry[deploymentID]
	if !ok {
		return Tenant{}, errTenantNotFound
	}
	return t, nil
}

func (s *fakeStore) ListTenants(_ context.Context) ([]Tenant, error) {
	out := make([]Tenant, 0, len(s.registry))
	for _, t := range s.registry {
		out = append(out, t)
	}
	return out, nil
}

func (s *fakeStore) DeleteTenantRecord(_ context.Context, deploymentID string) error {
	delete(s.registry, deploymentID)
	return nil
}

func (s *fakeStore) IsProvisioned(_ context.Context, deploymentID string) (bool, error) {
	return s.provisioned[deploymentID], nil
}

func (s *fakeStore) PurgeTenantData(_ context.Context, deploymentID string) error {
	s.purged = append(s.purged, deploymentID)
	delete(s.provisioned, deploymentID)
	return nil
}

func newTestService(store tenantStoreInterface, run func(context.Context, importer.ImportServiceInterface,
	bootstrap.Options) error) *tenantService {
	return &tenantService{
		store:              store,
		publicURL:          "https://cp.example",
		systemDeploymentID: systemID,
		bootstrapRun:       run,
	}
}

func systemCtx() context.Context {
	return deployment.WithID(context.Background(), systemID)
}

func TestCreateTenant_Success(t *testing.T) {
	store := newFakeStore()
	var provisioned []string
	svc := newTestService(store, func(_ context.Context, _ importer.ImportServiceInterface,
		opts bootstrap.Options) error {
		provisioned = append(provisioned, opts.DeploymentID)
		return nil
	})

	created, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "acme", Env: "dev", Name: "X"})

	require.Nil(t, svcErr)
	require.NotNil(t, created)
	assert.Equal(t, "acme", created.DeploymentID)
	assert.Equal(t, []string{"acme"}, provisioned)
	_, ok := store.registry["acme"]
	assert.True(t, ok)
}

// An organization whose name is the system tenant's own id is refused, whatever that id is
// configured to be.
func TestCreateTenant_ReservedSystemTenant(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	svc.systemDeploymentID = "acme"
	ctx := deployment.WithID(context.Background(), "acme")

	_, svcErr := svc.CreateTenant(ctx, CreateTenantRequest{Org: "acme", Env: "dev"})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorReservedSystemTenant.Code, svcErr.Code)
}

func TestCreateTenant_NotSystemCaller(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	ctx := deployment.WithID(context.Background(), "tenant-a")

	_, svcErr := svc.CreateTenant(ctx, CreateTenantRequest{Org: "acme", Env: "dev"})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorNotSystemTenant.Code, svcErr.Code)
}

func TestCreateTenant_Conflict(t *testing.T) {
	store := newFakeStore()
	store.provisioned["acme"] = true
	svc := newTestService(store, noopRun)

	_, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "acme", Env: "dev"})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorTenantConflict.Code, svcErr.Code)
}

func TestCreateTenant_InvalidDeploymentID(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	_, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "bad org!", Env: "dev"})

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorInvalidDeploymentID.Code, svcErr.Code)
}

func TestDeleteTenant_Success(t *testing.T) {
	store := newFakeStore()
	store.provisioned["tenant-x"] = true
	store.registry["tenant-x"] = Tenant{ID: "1", DeploymentID: "tenant-x"}
	svc := newTestService(store, noopRun)

	svcErr := svc.DeleteTenant(systemCtx(), "tenant-x")

	require.Nil(t, svcErr)
	assert.Equal(t, []string{"tenant-x"}, store.purged)
	_, ok := store.registry["tenant-x"]
	assert.False(t, ok)
}

func TestDeleteTenant_ReservedSystemTenant(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	svcErr := svc.DeleteTenant(systemCtx(), systemID)

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorReservedSystemTenant.Code, svcErr.Code)
}

func TestDeleteTenant_NotFound(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)

	svcErr := svc.DeleteTenant(systemCtx(), "ghost")

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorTenantNotFound.Code, svcErr.Code)
}

func TestListTenants_RequiresSystemCaller(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	ctx := deployment.WithID(context.Background(), "tenant-a")

	_, svcErr := svc.ListTenants(ctx)

	require.NotNil(t, svcErr)
	assert.Equal(t, ErrorNotSystemTenant.Code, svcErr.Code)
}

func TestListTenants_Success(t *testing.T) {
	store := newFakeStore()
	store.registry["tenant-x"] = Tenant{ID: "1", DeploymentID: "tenant-x"}
	store.registry["tenant-y"] = Tenant{ID: "2", DeploymentID: "tenant-y"}
	svc := newTestService(store, noopRun)

	resp, svcErr := svc.ListTenants(systemCtx())

	require.Nil(t, svcErr)
	assert.Equal(t, 2, resp.TotalResults)
	assert.Len(t, resp.Tenants, 2)
}

func noopRun(_ context.Context, _ importer.ImportServiceInterface, _ bootstrap.Options) error {
	return nil
}

// stubSeeder records what a later environment was copied from.
type stubSeeder struct {
	registered *RegisterEnvironmentInput
}

func (s *stubSeeder) RegisterEnvironment(_ context.Context,
	in RegisterEnvironmentInput) (*EnvironmentSummary, error) {
	s.registered = &in
	return &EnvironmentSummary{ID: "env-1", Name: in.Name, Rank: in.Rank}, nil
}

// Every deployment is provisioned from the baseline bundle. Nothing is copied from a sibling: an
// organization has one workspace, and its environments are resources inside it.
func TestCreateTenant_IsProvisionedFromTheBaseline(t *testing.T) {
	store := newFakeStore()
	store.registry["other:dev"] = Tenant{ID: "1", DeploymentID: "other:dev", CreatedAt: "2026-01-01T00:00:00Z"}
	var provisioned []string
	svc := newTestService(store, func(_ context.Context, _ importer.ImportServiceInterface,
		opts bootstrap.Options) error {
		provisioned = append(provisioned, opts.DeploymentID)
		return nil
	})
	seeder := &stubSeeder{}
	svc.SetBaselineSeeder(seeder)

	created, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "acme", Env: "dev"})

	require.Nil(t, svcErr)
	assert.Equal(t, []string{"acme"}, provisioned)
	_ = created
}

// A tenant that names a data plane is registered for promotion as it is created, so an environment
// does not have to be set up in a second step that is easy to forget.
func TestCreateTenant_RegistersTheEnvironmentWithItsRank(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	seeder := &stubSeeder{}
	svc.SetBaselineSeeder(seeder)

	created, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{
		Org: "acme", Env: "dev",
		DataPlane:    &DataPlane{ID: "dev-dp", BaseURL: "https://dp-dev"},
		ControlPlane: &ControlPlane{InsecureSkipVerify: true},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, seeder.registered)
	assert.Equal(t, "dev", seeder.registered.Name)
	// The deployment is the organization: its environments are resources inside that one workspace.
	assert.Equal(t, "acme", seeder.registered.DeploymentID)
	assert.Equal(t, 1, seeder.registered.Rank)
	assert.Equal(t, "dev-dp", seeder.registered.DataPlane.ID)
	require.NotNil(t, created.Environment)
	assert.Equal(t, 1, created.Environment.Rank)
}

// Creating an organization registers its first environment, which is the bottom of the promotion
// chain, so its rank is 1 whatever the caller asks for.
func TestCreateTenant_FirstEnvironmentIsAlwaysRankOne(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	seeder := &stubSeeder{}
	svc.SetBaselineSeeder(seeder)
	rank := 7

	_, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{
		Org: "acme", Env: "dev", Rank: &rank,
		DataPlane: &DataPlane{ID: "dev-dp", BaseURL: "https://dp-dev"},
	})

	require.Nil(t, svcErr)
	require.NotNil(t, seeder.registered)
	assert.Equal(t, 1, seeder.registered.Rank)
}

// Without a data plane there is nowhere to apply to, so only the tenant is created.
func TestCreateTenant_WithoutADataPlaneRegistersNoEnvironment(t *testing.T) {
	svc := newTestService(newFakeStore(), noopRun)
	seeder := &stubSeeder{}
	svc.SetBaselineSeeder(seeder)

	created, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "acme", Env: "dev"})

	require.Nil(t, svcErr)
	assert.Nil(t, seeder.registered)
	assert.Nil(t, created.Environment)
}

// A tenant created before its data plane existed has no environment. Registering one later must be
// possible with the system credentials the tenant was created with, rather than a token for the
// tenant itself, which is what a platform provisioning it from outside actually holds.
func TestRegisterEnvironment_RegistersAnExistingTenant(t *testing.T) {
	store := newFakeStore()
	seeder := &stubSeeder{}
	svc := newTestService(store, func(context.Context, importer.ImportServiceInterface,
		bootstrap.Options) error {
		return nil
	})
	svc.SetBaselineSeeder(seeder)

	// A tenant with no data plane: created, but with nowhere to apply to.
	if _, svcErr := svc.CreateTenant(systemCtx(), CreateTenantRequest{Org: "acme", Env: "dev"}); svcErr != nil {
		t.Fatalf("create tenant: %v", svcErr.Error.DefaultValue)
	}
	if seeder.registered != nil {
		t.Fatal("expected no environment without a data plane")
	}

	env, svcErr := svc.RegisterEnvironment(systemCtx(), "acme", RegisterEnvironmentRequest{
		Env:       "dev",
		DataPlane: DataPlane{ID: "acme:dev", BaseURL: "https://dev.example"},
	})
	if svcErr != nil {
		t.Fatalf("register environment: %v", svcErr.Error.DefaultValue)
	}
	if env == nil || seeder.registered == nil {
		t.Fatal("expected the environment to be registered")
	}
	if seeder.registered.DeploymentID != "acme" {
		t.Fatalf("expected it registered against the organization, got %q", seeder.registered.DeploymentID)
	}
	if seeder.registered.Name != "dev" {
		t.Fatalf("expected the environment named by the request, got %q", seeder.registered.Name)
	}
}

// Only the system tenant provisions, and only a tenant that exists can be registered.
func TestRegisterEnvironment_Refusals(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store, func(context.Context, importer.ImportServiceInterface,
		bootstrap.Options) error {
		return nil
	})
	svc.SetBaselineSeeder(&stubSeeder{})

	req := RegisterEnvironmentRequest{Env: "dev", DataPlane: DataPlane{ID: "acme:dev"}}

	if _, svcErr := svc.RegisterEnvironment(deployment.WithID(context.Background(), "acme"),
		"acme", req); svcErr == nil {
		t.Fatal("expected a tenant's own token to be refused")
	}
	if _, svcErr := svc.RegisterEnvironment(systemCtx(), "missing", req); svcErr == nil {
		t.Fatal("expected an unknown organization to be refused")
	}
	if _, svcErr := svc.RegisterEnvironment(systemCtx(), systemID, req); svcErr == nil {
		t.Fatal("expected the system tenant to be refused")
	}
	if _, svcErr := svc.RegisterEnvironment(systemCtx(), "acme",
		RegisterEnvironmentRequest{}); svcErr == nil {
		t.Fatal("expected a missing data plane to be refused")
	}
}
