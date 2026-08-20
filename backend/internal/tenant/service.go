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
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/bootstrap"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const tenantLoggerComponentName = "TenantService"

// deploymentIDPattern restricts a managed tenant's deployment id to a safe, portable character set.
var deploymentIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

// orgEnvPattern restricts an organization and environment name. It excludes the separator, so a
// deployment id always splits back into the pair it was built from.
var orgEnvPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// BaselineSeeder copies an organization's existing configuration into a newly created tenant.
//
// It is supplied by the server rather than built here, because the configuration comes from the
// environment manager, which is hosted alongside this service rather than owned by it. Without one, a
// second environment is created empty and is populated by the first promotion into it.
type BaselineSeeder interface {
	// RegisterEnvironment records the tenant as an environment of its organization, so it takes part
	// in promotion without a second call to set it up.
	RegisterEnvironment(ctx context.Context, in RegisterEnvironmentInput) (*EnvironmentSummary, error)
}

// RegisterEnvironmentInput describes the promotion entry a new tenant is registered as.
type RegisterEnvironmentInput struct {
	// Name is the environment's name within its organization, e.g. "dev".
	Name string
	// DeploymentID is the tenant this environment's configuration is held in.
	DeploymentID string
	// Rank orders it in the promotion chain. Zero means the end of the chain.
	Rank      int
	DataPlane DataPlane
}

// TenantServiceInterface defines platform tenant-management operations, usable only by the system
// tenant.
type TenantServiceInterface interface {
	CreateTenant(ctx context.Context, request CreateTenantRequest) (*CreateTenantResponse, *tidcommon.ServiceError)
	ListTenants(ctx context.Context) (*TenantListResponse, *tidcommon.ServiceError)
	DeleteTenant(ctx context.Context, deploymentID string) *tidcommon.ServiceError
	// RegisterEnvironment registers an existing tenant as an environment of its organization, for a
	// tenant created before its data plane existed.
	RegisterEnvironment(ctx context.Context, deploymentID string,
		request RegisterEnvironmentRequest) (*EnvironmentSummary, *tidcommon.ServiceError)
	// SetBaselineSeeder installs what a later environment of an organization is copied from.
	SetBaselineSeeder(seeder BaselineSeeder)
}

// tenantService is the default implementation of TenantServiceInterface.
type tenantService struct {
	store              tenantStoreInterface
	importSvc          importer.ImportServiceInterface
	defaultsDir        string
	publicURL          string
	systemDeploymentID string
	// bootstrapRun provisions a tenant's baseline. It defaults to bootstrap.Run and is a field so
	// tests can substitute it.
	bootstrapRun func(ctx context.Context, importSvc importer.ImportServiceInterface, opts bootstrap.Options) error
	// seeder copies an organization's configuration into its later environments.
	seeder BaselineSeeder
	// provisionMu serializes provisioning because it sets process-global env vars for the bootstrap
	// bundle's template substitution and runs the bootstrap import.
	provisionMu sync.Mutex
}

// SetBaselineSeeder installs what a later environment of an organization is copied from. It is set
// after the fact because the environment manager is built after this service.
func (s *tenantService) SetBaselineSeeder(seeder BaselineSeeder) {
	s.seeder = seeder
}

func newTenantService(store tenantStoreInterface, importSvc importer.ImportServiceInterface,
	defaultsDir, publicURL, systemDeploymentID string) TenantServiceInterface {
	return &tenantService{
		store:              store,
		importSvc:          importSvc,
		defaultsDir:        defaultsDir,
		publicURL:          publicURL,
		systemDeploymentID: systemDeploymentID,
		bootstrapRun:       bootstrap.Run,
	}
}

// requireSystemTenant ensures the caller belongs to the system tenant (its token carries the system
// deployment id). This is what makes tenant management exclusive to the system tenant.
func (s *tenantService) requireSystemTenant(ctx context.Context) *tidcommon.ServiceError {
	id, ok := deployment.IDFromContext(ctx)
	if !ok || id != s.systemDeploymentID {
		return &ErrorNotSystemTenant
	}
	return nil
}

// CreateTenant provisions an organization's environment and records it in the registry.
//
// The organization's first environment is provisioned from the bootstrap baseline. Every later one is
// created empty and seeded from the first, so an organization's environments hold the same resources
// under the same ids and configuration can be promoted between them. Provisioning each from the
// baseline instead would give every environment its own organization unit, user types and themes, and
// a promotion would then collide with them or name ids the destination has never had.
func (s *tenantService) CreateTenant(ctx context.Context,
	request CreateTenantRequest) (*CreateTenantResponse, *tidcommon.ServiceError) {
	if svcErr := s.requireSystemTenant(ctx); svcErr != nil {
		return nil, svcErr
	}
	if !orgEnvPattern.MatchString(request.Org) || !orgEnvPattern.MatchString(request.Env) {
		return nil, &ErrorInvalidDeploymentID
	}
	// The deployment is the organization. Its environments are resources inside that one workspace
	// rather than deployments of their own, so a second environment adds no deployment.
	deploymentID := request.Org
	if deploymentID == s.systemDeploymentID {
		return nil, &ErrorReservedSystemTenant
	}
	if !deploymentIDPattern.MatchString(deploymentID) {
		return nil, &ErrorInvalidDeploymentID
	}

	provisioned, err := s.store.IsProvisioned(ctx, deploymentID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to check tenant provisioning state", err)
	}
	if provisioned {
		return nil, &ErrorTenantConflict
	}

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, s.internalError(ctx, "failed to generate tenant id", err)
	}

	// A deployment is provisioned from the baseline bundle. Nothing is copied from a sibling: an
	// organization has one workspace, and its environments are resources inside it rather than
	// deployments of their own.
	if svcErr := s.provision(ctx, deploymentID); svcErr != nil {
		return nil, svcErr
	}

	// This is the organization's first environment: the deployment did not exist a moment ago.
	rank := 1
	environment, svcErr := s.registerEnvironment(ctx, request, deploymentID, rank)
	if svcErr != nil {
		return nil, svcErr
	}

	tenant := Tenant{ID: id, DeploymentID: deploymentID, Name: request.Name}
	if err := s.store.CreateTenant(ctx, tenant); err != nil {
		return nil, s.internalError(ctx, "failed to record tenant", err)
	}
	return &CreateTenantResponse{Tenant: tenant, Environment: environment}, nil
}

// RegisterEnvironment registers an existing tenant as an environment of its organization.
//
// It exists because a tenant can be created before its data plane does, and the environment cannot be
// registered until there is one to apply to. Doing it here rather than through the environment API
// means the platform can complete the setup with the same system credentials it created the tenant
// with, instead of needing a token for that tenant.
func (s *tenantService) RegisterEnvironment(ctx context.Context, deploymentID string,
	request RegisterEnvironmentRequest) (*EnvironmentSummary, *tidcommon.ServiceError) {
	if svcErr := s.requireSystemTenant(ctx); svcErr != nil {
		return nil, svcErr
	}
	if strings.TrimSpace(request.DataPlane.ID) == "" {
		return nil, &ErrorInvalidDataPlane
	}
	if deploymentID == s.systemDeploymentID {
		return nil, &ErrorReservedSystemTenant
	}

	provisioned, err := s.store.IsProvisioned(ctx, deploymentID)
	if err != nil {
		return nil, s.internalError(ctx, "failed to check tenant provisioning state", err)
	}
	if !provisioned {
		return nil, &ErrorTenantNotFound
	}
	if s.seeder == nil {
		return nil, &ErrorEnvironmentRegistrationUnavailable
	}

	rank := 0
	if request.Rank != nil {
		rank = *request.Rank
	}

	summary, err := s.seeder.RegisterEnvironment(ctx, RegisterEnvironmentInput{
		Name:         request.Env,
		DeploymentID: deploymentID,
		Rank:         rank,
		DataPlane:    request.DataPlane,
	})
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantLoggerComponentName)).
			Error(ctx, "Failed to register the tenant as an environment",
				log.String("deploymentId", deploymentID), log.Error(err))
		svcErr := ErrorEnvironmentRegistrationFailed
		svcErr.ErrorDescription = tidcommon.I18nMessage{
			Key:          "error.tenantservice.environment_registration_failed_description",
			DefaultValue: err.Error(),
		}
		return nil, &svcErr
	}
	return summary, nil
}

// registerEnvironment records the new tenant as an environment of its organization. Without a data
// plane to apply to there is no environment to register, which is not an error: the tenant is usable
// and the environment can be registered once its data plane exists.
func (s *tenantService) registerEnvironment(ctx context.Context, request CreateTenantRequest,
	deploymentID string, rank int) (*EnvironmentSummary, *tidcommon.ServiceError) {
	if request.DataPlane == nil || strings.TrimSpace(request.DataPlane.ID) == "" {
		return nil, nil
	}
	if s.seeder == nil {
		return nil, nil
	}

	summary, err := s.seeder.RegisterEnvironment(ctx, RegisterEnvironmentInput{
		Name:         request.Env,
		DeploymentID: deploymentID,
		Rank:         rank,
		DataPlane:    *request.DataPlane,
	})
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantLoggerComponentName)).
			Error(ctx, "Failed to register the tenant as an environment",
				log.String("deploymentId", deploymentID), log.Error(err))
		svcErr := ErrorEnvironmentRegistrationFailed
		svcErr.ErrorDescription = tidcommon.I18nMessage{
			Key:          "error.tenantservice.environment_registration_failed_description",
			DefaultValue: err.Error(),
		}
		return nil, &svcErr
	}
	return summary, nil
}

// provision runs the bootstrap import scoped to the target deployment id. It is serialized because it
// sets process-global env vars that the bootstrap bundle's placeholders resolve from.
func (s *tenantService) provision(ctx context.Context, deploymentID string) *tidcommon.ServiceError {
	s.provisionMu.Lock()
	defer s.provisionMu.Unlock()

	// No administrator credentials: a tenant is provisioned without a local administrator, because
	// whoever administers it signs in against the trusted issuer instead.
	for key, value := range map[string]string{
		"PUBLIC_URL":              s.publicURL,
		"CONSOLE_REDIRECT_URIS_0": s.publicURL + "/console",
	} {
		if err := os.Setenv(key, value); err != nil {
			return s.internalError(ctx, "failed to set bootstrap environment", err)
		}
	}

	if err := s.bootstrapRun(ctx, s.importSvc, bootstrap.Options{
		DefaultsDir:  s.defaultsDir,
		DeploymentID: deploymentID,
	}); err != nil {
		return s.internalError(ctx, "failed to provision tenant baseline", err)
	}
	return nil
}

// ListTenants returns all managed tenants.
func (s *tenantService) ListTenants(ctx context.Context) (*TenantListResponse, *tidcommon.ServiceError) {
	if svcErr := s.requireSystemTenant(ctx); svcErr != nil {
		return nil, svcErr
	}
	tenants, err := s.store.ListTenants(ctx)
	if err != nil {
		return nil, s.internalError(ctx, "failed to list tenants", err)
	}
	return &TenantListResponse{TotalResults: len(tenants), Count: len(tenants), Tenants: tenants}, nil
}

// DeleteTenant deprovisions a tenant: purges all of its data and removes its registry row. The system
// tenant itself cannot be deleted.
func (s *tenantService) DeleteTenant(ctx context.Context, deploymentID string) *tidcommon.ServiceError {
	if svcErr := s.requireSystemTenant(ctx); svcErr != nil {
		return svcErr
	}
	if deploymentID == s.systemDeploymentID {
		return &ErrorReservedSystemTenant
	}

	provisioned, err := s.store.IsProvisioned(ctx, deploymentID)
	if err != nil {
		return s.internalError(ctx, "failed to check tenant provisioning state", err)
	}
	if !provisioned {
		if _, getErr := s.store.GetTenant(ctx, deploymentID); errors.Is(getErr, errTenantNotFound) {
			return &ErrorTenantNotFound
		}
	}

	if err := s.store.PurgeTenantData(ctx, deploymentID); err != nil {
		return s.internalError(ctx, "failed to purge tenant data", err)
	}
	if err := s.store.DeleteTenantRecord(ctx, deploymentID); err != nil {
		return s.internalError(ctx, "failed to delete tenant record", err)
	}
	return nil
}

// internalError logs the underlying error and returns the generic server-side ServiceError.
func (s *tenantService) internalError(ctx context.Context, msg string, err error) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, tenantLoggerComponentName))
	logger.Error(ctx, msg, log.Error(err))
	return &ErrorInternalServer
}
