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
	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnconsent "github.com/thunder-id/thunderid/internal/authn/consent"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/observability"
)

// Config is the engine's server configuration, re-exported so embedders can seed the runtime
// configuration through WithConfig without importing internal/* packages.
type Config = config.Config

// Public aliases for the dependencies an embedding application supplies to the engine.
// They re-export internal interfaces so embedders do not import internal/* directly.
type (
	// ActorProvider resolves actors (users/clients) for flow and OAuth flows.
	ActorProvider = actorprovider.ActorProviderInterface
	// AuthnProvider performs authentication operations during flow execution.
	AuthnProvider = authnprovidermgr.AuthnProviderManagerInterface
	// OUService resolves organization units.
	OUService = ou.OrganizationUnitServiceInterface
	// AttributeCacheService stores/retrieves short-lived attribute caches.
	AttributeCacheService = attributecache.AttributeCacheServiceInterface
	// AuthZService performs authorization decisions.
	AuthZService = authz.AuthorizationServiceInterface
	// RoleService resolves roles and permissions for authorization and token assertion.
	RoleService = role.RoleServiceInterface
	// ResourceService resolves protected resources/scopes.
	ResourceService = resource.ResourceServiceInterface
	// I18nService resolves translations.
	I18nService = i18nmgt.I18nServiceInterface
	// IDPService resolves identity provider configuration.
	IDPService = idp.IDPServiceInterface
	// FlowProvider supplies flow definitions to the execution engine.
	FlowProvider = flowexec.FlowProviderInterface
	// ExecutorRegistry resolves flow executors by name.
	ExecutorRegistry = core.ExecutorRegistryInterface
	// DesignResolveService resolves theme/layout for flow metadata.
	DesignResolveService = resolve.DesignResolveServiceInterface
	// JWTService signs/validates JWTs.
	JWTService = jwt.JWTServiceInterface
	// JWEService encrypts/decrypts JWEs.
	JWEService = jwe.JWEServiceInterface
	// ObservabilityService publishes observability events.
	ObservabilityService = observability.ObservabilityServiceInterface
	// RuntimeCryptoProvider supplies runtime signing/crypto material.
	RuntimeCryptoProvider = kmprovider.RuntimeCryptoProvider
)

// Public aliases for the executor-registry build path. Supply ExecutorDependencies via
// WithExecutorDependencies (instead of WithExecutorRegistry) to have the engine construct
// the flow executor registry itself, so an embedder can inject custom dependencies — most
// notably its own ConsentEnforcer — without importing internal/* packages.
type (
	// ExecutorDependencies holds the service dependencies used to construct built-in flow
	// executors. Only the fields required by the executors named in WithEnabledExecutors need
	// to be set; the engine fills FlowFactory and reuses the dependencies it already holds
	// (actor, authn, OU, JWT, attribute-cache, authz, and IDP providers, plus the
	// auth-assertion generator) when the matching field is left nil.
	ExecutorDependencies = executor.ExecutorDependencies
	// ConsentEnforcer resolves and records user consent during flow execution. Supply a custom
	// implementation through ExecutorDependencies when enabling the consent executor.
	ConsentEnforcer = authnconsent.ConsentEnforcerServiceInterface
	// ConsentPromptData describes the consent a user still needs to provide.
	ConsentPromptData = authnconsent.ConsentPromptData
	// ConsentDecisions carries a user's consent decisions to be recorded.
	ConsentDecisions = authnconsent.ConsentDecisions
	// ConsentRecord is a persisted consent record returned after recording decisions.
	ConsentRecord = consent.Consent
	// AttributesResponse carries the attribute set available for consent resolution.
	AttributesResponse = authnprovidercm.AttributesResponse
	// ServiceError is the engine's structured service-error type.
	ServiceError = serviceerror.ServiceError
)

// Public aliases for authoring a custom flow executor. They re-export the internal flow types
// referenced by ExecutorInterface so an embedder can implement an executor and register it via
// WithCustomExecutors without importing internal/* packages. Embed a NewBaseExecutor value to
// inherit the boilerplate methods and override only Execute.
type (
	// ExecutorInterface is the contract a flow executor implements. A custom executor is
	// registered with WithCustomExecutors and referenced by name from a flow TASK node.
	ExecutorInterface = core.ExecutorInterface
	// ExecutorNodeContext is the per-node execution context passed to ExecutorInterface.Execute.
	ExecutorNodeContext = core.NodeContext
	// ExecutorExecutionPolicy describes how the engine drives an executor for a given mode.
	ExecutorExecutionPolicy = core.ExecutionPolicy
	// ExecutorResponse is the result an executor returns from Execute.
	ExecutorResponse = flowcommon.ExecutorResponse
	// ExecutorType classifies an executor (for example authentication or utility).
	ExecutorType = flowcommon.ExecutorType
	// ExecutorInput describes an input an executor requires or provides.
	ExecutorInput = flowcommon.Input
	// ExecutorStatus is the status an executor reports in its ExecutorResponse.
	ExecutorStatus = flowcommon.ExecutorStatus
)

// Executor type values for NewBaseExecutor.
const (
	// ExecutorTypeAuthentication marks an executor that performs authentication.
	ExecutorTypeAuthentication = flowcommon.ExecutorTypeAuthentication
	// ExecutorTypeUtility marks a utility executor for common operations.
	ExecutorTypeUtility = flowcommon.ExecutorTypeUtility
)

// Executor status values for an ExecutorResponse returned from Execute.
const (
	// ExecComplete indicates the executor finished successfully.
	ExecComplete = flowcommon.ExecComplete
	// ExecUserInputRequired indicates the executor needs further user input.
	ExecUserInputRequired = flowcommon.ExecUserInputRequired
	// ExecExternalRedirection indicates the executor requires an external redirect.
	ExecExternalRedirection = flowcommon.ExecExternalRedirection
)

// NewBaseExecutor returns a base ExecutorInterface that supplies the boilerplate executor
// methods (name, type, inputs, prerequisites, default execution policy). Embed the returned
// value in a custom executor struct and override Execute — mirroring how the built-in executors
// are constructed. Register the resulting executor with WithCustomExecutors.
func NewBaseExecutor(name string, executorType ExecutorType,
	defaultInputs, prerequisites []ExecutorInput) ExecutorInterface {
	return core.NewBaseExecutor(name, executorType, defaultInputs, prerequisites)
}
