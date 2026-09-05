// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// clientSecretExecutor acts on the client secret of the application named in the trusted revocation
// plan. It regenerates the secret, and runs after the artifacts issued under the old secret have been
// denied.
//
// The name is action-neutral while the behavior is not. An executor name is persisted into stored
// flow definitions, so renaming this one once deployments have authored flows against it would break
// them; the further actions on a client secret therefore arrive as executor modes under this name.
// Whichever adds the first mode picks one of the two shapes already in this package: declare no
// DefaultMode so a node must name its mode, as EmailExecutor does, or declare one and let the switch
// treat the unset mode as that default, as IdentifyingExecutor does. DefaultMode alone is not enough,
// because it is read by flow validation only and the graph builder hands the executor the node's own
// mode, which stays empty.
type clientSecretExecutor struct {
	providers.Executor
	provider providers.ApplicationAdminProvider
}

var _ providers.Executor = (*clientSecretExecutor)(nil)

// newClientSecretExecutor creates an executor that regenerates an application's client secret.
func newClientSecretExecutor(factory core.FlowFactoryInterface) *clientSecretExecutor {
	base := factory.CreateExecutor(ExecutorNameClientSecret, providers.ExecutorTypeUtility,
		nil, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &clientSecretExecutor{Executor: base}
}

// Execute regenerates the secret and returns the new value to the caller.
//
// The value goes out on AdditionalData, the only executor output the engine serializes, and this is the
// single moment it is readable: the entity layer hashes it on write and no read path returns it. It is
// deliberately not placed on RuntimeData, which persists with the flow context.
func (e *clientSecretExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	provider, err := e.applicationProvider()
	if err != nil {
		return nil, err
	}
	appID, err := applicationTargetFromPlan(
		ctx.SharedRuntimeData, revocation.ReasonApplicationSecretRegenerated)
	if err != nil {
		return nil, err
	}

	secret, svcErr := provider.ApplyCredentialAction(
		ctx.Context, appID, providers.CredentialActionRegenerate)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to regenerate client secret: %s", svcErr.Error.DefaultValue)
		}
		return &providers.ExecutorResponse{
			Status: providers.ExecFailure,
			Error:  &ErrSecretRegenerationFailed,
		}, nil
	}
	return &providers.ExecutorResponse{
		Status:         providers.ExecComplete,
		AdditionalData: map[string]string{dataKeyClientSecret: secret},
	}, nil
}

// setApplicationProvider injects the application service. See ExecutorRegistryInterface.
func (e *clientSecretExecutor) setApplicationProvider(provider providers.ApplicationAdminProvider) {
	e.provider = provider
}

// applicationProvider returns the injected application service, or an error when the second-phase
// injection never ran.
func (e *clientSecretExecutor) applicationProvider() (providers.ApplicationAdminProvider, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("application service is not configured")
	}
	return e.provider, nil
}
