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

package thunderidengine_test

import (
	"context"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/runtime"
)

// exampleActorProvider is a minimal host.ActorProvider backed by the embedder's own identity
// source. Reads that find no record return runtime.ErrNotFound, which the engine treats as a
// normal "absent" result. A real implementation would query the embedder's store.
type exampleActorProvider struct{}

func (exampleActorProvider) IdentifyEntity(map[string]any) (*string, error) {
	return nil, runtime.ErrNotFound
}
func (exampleActorProvider) GetEntity(string) (*host.Actor, error) { return nil, runtime.ErrNotFound }
func (exampleActorProvider) SearchEntities(map[string]any) ([]*host.Actor, error) {
	return nil, nil
}
func (exampleActorProvider) GetApplication(context.Context, string) (*host.Application, error) {
	return nil, runtime.ErrNotFound
}
func (exampleActorProvider) GetInboundClientByEntityID(
	context.Context, string,
) (*host.InboundClient, error) {
	return nil, runtime.ErrNotFound
}
func (exampleActorProvider) GetInboundClientByClientID(
	context.Context, string,
) (*host.InboundClient, error) {
	return nil, runtime.ErrNotFound
}
func (exampleActorProvider) GetEntityType(context.Context, string) (*host.EntityType, error) {
	return nil, runtime.ErrNotFound
}

// exampleAuthnProvider is a minimal host.AuthnProvider. A real implementation would verify the
// supplied credentials against the embedder's identity source.
type exampleAuthnProvider struct{}

func (exampleAuthnProvider) Authenticate(
	context.Context, map[string]any, map[string]any, *host.AuthnMetadata,
) (*host.AuthnResult, error) {
	return &host.AuthnResult{Authenticated: false}, nil
}
func (exampleAuthnProvider) GetAttributes(
	context.Context, string, *host.RequestedAttributes, *host.GetAttributesMetadata,
) (*host.GetAttributesResult, error) {
	return &host.GetAttributesResult{}, nil
}

// exampleRoleProvider is a minimal host.RoleProvider for authorization and token assertions.
type exampleRoleProvider struct{}

func (exampleRoleProvider) GetAuthorizedPermissions(
	_ context.Context, _ string, _ []string, requested []string,
) ([]string, error) {
	return requested, nil
}

func (exampleRoleProvider) GetUserRoles(_ context.Context, _ string, _ []string) ([]string, error) {
	return nil, nil
}

// greetExecutor is a custom flow executor.
// inherits the boilerplate ExecutorInterface methods, and overrides only Execute.
type greetExecutor struct {
	thunderidengine.ExecutorInterface
}

func (*greetExecutor) Execute(
	*thunderidengine.ExecutorNodeContext,
) (*thunderidengine.ExecutorResponse, error) {
	return &thunderidengine.ExecutorResponse{Status: thunderidengine.ExecComplete}, nil
}

// Example shows an external Go application embedding the engine with no SQL database: runtime
// state lives in Redis, identity comes from host providers, the system-of-record services fall
// back to declarative (file-based) resources, a subset of the built-in executors is enabled, and
// a custom executor is registered alongside them.
func Example() {
	const serverHome = "/etc/thunderid"

	// Runtime state store. The caller owns the client's lifecycle.
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})

	// Engine defaults omit database configuration; only deployment.yaml at serverHome is required.
	cfg, err := thunderidengine.LoadEngineConfig(serverHome)
	if err != nil {
		log.Fatal(err)
	}

	eng, err := thunderidengine.New(
		thunderidengine.WithRedis(rdb, "thunderid:"),
		thunderidengine.WithConfig(serverHome, cfg),
		// Derive crypto/JWT/JWE from a PEM key pair (relative to serverHome).
		thunderidengine.WithPKIKey("default", "certs/server.crt", "certs/server.key"),
		// Embedder-specific identity. EntityProvider duties are served through the ActorProvider.
		thunderidengine.WithHostActorProvider(exampleActorProvider{}),
		thunderidengine.WithHostAuthnProvider(exampleAuthnProvider{}),
		thunderidengine.WithHostRoleProvider(exampleRoleProvider{}),
		thunderidengine.WithExecutorDependencies(thunderidengine.ExecutorDependencies{}),
		// Enable a subset of the built-in executors.
		thunderidengine.WithEnabledExecutors(
			"CredentialsAuthExecutor",
			"AuthorizationExecutor",
			"AuthAssertExecutor",
			"ConsentExecutor",
		),
		// Register a custom executor that runs alongside the enabled built-ins.
		thunderidengine.WithCustomExecutors(map[string]thunderidengine.ExecutorInterface{
			"GreetExecutor": &greetExecutor{
				ExecutorInterface: thunderidengine.NewBaseExecutor(
					"GreetExecutor", thunderidengine.ExecutorTypeUtility, nil, nil),
			},
		}),
		// SDK-required SoR services not injected above fall back to declarative file resources.
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = eng.Shutdown(context.Background()) }()

	handler, err := eng.Handler()
	if err != nil {
		log.Fatal(err)
	}
	// Serves GET /flow/meta, POST /flow/execute, and /oauth2/*.
	log.Fatal(http.ListenAndServe(":9443", handler)) //nolint:gosec // example only
}
