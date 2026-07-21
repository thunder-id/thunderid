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

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authncommon "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

// fakeOpenID4VPService is a test double for openid4vp.OpenID4VPServiceInterface.
type fakeOpenID4VPService struct {
	initiate  func(ctx context.Context, definitionID string) (*openid4vp.Initiation, *tidcommon.ServiceError)
	getResult func(ctx context.Context, state string) (*openid4vp.RequestState, *tidcommon.ServiceError)
}

func (f *fakeOpenID4VPService) Initiate(
	ctx context.Context, definitionID string,
) (*openid4vp.Initiation, *tidcommon.ServiceError) {
	return f.initiate(ctx, definitionID)
}

func (f *fakeOpenID4VPService) GetResult(
	ctx context.Context, state string,
) (*openid4vp.RequestState, *tidcommon.ServiceError) {
	return f.getResult(ctx, state)
}

func (f *fakeOpenID4VPService) Authenticate(
	_ context.Context, _ *authncommon.OpenID4VPCredential,
) (*authncommon.AuthnResult, *tidcommon.ServiceError) {
	return nil, nil
}

func newTestOpenID4VPExecutor(t *testing.T, service openid4vp.OpenID4VPServiceInterface) providers.Executor {
	t.Helper()
	return newTestOpenID4VPExecutorWithProvider(t, service, nil)
}

func newTestOpenID4VPExecutorWithProvider(t *testing.T, service openid4vp.OpenID4VPServiceInterface,
	authnProvider providers.AuthnProviderManager) providers.Executor {
	t.Helper()
	factory := coremock.NewFlowFactoryInterfaceMock(t)
	base := coremock.NewExecutorInterfaceMock(t)
	factory.On("CreateExecutor", ExecutorNameOpenID4VPVerify, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, mock.Anything).Return(base).Maybe()
	return newOpenID4VPVerifier(factory, service, authnProvider)
}

func openid4vpNodeContext(runtime map[string]string, properties map[string]interface{}) *providers.NodeContext {
	if runtime == nil {
		runtime = map[string]string{}
	}
	return &providers.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "exec-1",
		FlowType:       providers.FlowTypeAuthentication,
		RuntimeData:    runtime,
		NodeProperties: properties,
	}
}

func TestOpenID4VPExecutorInitiates(t *testing.T) {
	var seenDefID string
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, defID string) (*openid4vp.Initiation, *tidcommon.ServiceError) {
			seenDefID = defID
			return &openid4vp.Initiation{
				State:      "state-123",
				ClientID:   "x509_hash:abc",
				RequestURI: "https://verifier.example/openid4vp/request?state=state-123",
				WalletURI: "openid4vp://authorize?client_id=x509_hash%3Aabc" +
					"&request_uri=https%3A%2F%2Fverifier.example%2Fopenid4vp%2Frequest%3Fstate%3Dstate-123",
			}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	props := map[string]interface{}{propertyKeyPresentationDefinitionID: "custom-def"}
	resp, err := exec.Execute(openid4vpNodeContext(nil, props))
	require.NoError(t, err)
	assert.Equal(t, "custom-def", seenDefID, "executor must pass the configured definition id")
	assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
	assert.Equal(t, "state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	assert.Equal(t, "x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

// When no presentation_definition_id is configured on the node, the executor
// fails with a configuration error.
func TestOpenID4VPExecutorMissingDefinitionID(t *testing.T) {
	called := false
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, _ string) (*openid4vp.Initiation, *tidcommon.ServiceError) {
			called = true
			return &openid4vp.Initiation{State: "s", ClientID: "x509_hash:abc", RequestURI: "https://x",
				WalletURI: "openid4vp://authorize"}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	resp, err := exec.Execute(openid4vpNodeContext(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPDefinitionNotConfigured.Code, resp.Error.Code)
	assert.False(t, called, "initiate must not be called when definition id is missing")
}

func TestOpenID4VPExecutorInitiateFailure(t *testing.T) {
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, _ string) (*openid4vp.Initiation, *tidcommon.ServiceError) {
			return nil, &tidcommon.InternalServerError
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	resp, err := exec.Execute(openid4vpNodeContext(nil, map[string]interface{}{
		propertyKeyPresentationDefinitionID: "custom-def",
	}))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPInitiateFailed.Code, resp.Error.Code)
}

func TestOpenID4VPExecutorPollPending(t *testing.T) {
	svc := &fakeOpenID4VPService{
		getResult: func(_ context.Context, state string) (*openid4vp.RequestState, *tidcommon.ServiceError) {
			return &openid4vp.RequestState{
				Status:     openid4vp.StatusPending,
				ClientID:   "x509_hash:abc",
				RequestURI: "https://verifier.example/openid4vp/request?state=" + state,
			}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := exec.Execute(openid4vpNodeContext(runtime, nil))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
	assert.Equal(t, "state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	// QR data must persist across polls so the wait view keeps rendering it.
	assert.Equal(t, "x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

func TestOpenID4VPExecutorPollCompleted(t *testing.T) {
	svc := &fakeOpenID4VPService{
		getResult: func(_ context.Context, _ string) (*openid4vp.RequestState, *tidcommon.ServiceError) {
			return &openid4vp.RequestState{
				Status: openid4vp.StatusCompleted,
				Result: &openid4vp.VerifiedPresentation{
					Subject: "sub-1",
					Claims: map[string]interface{}{
						"given_name":  "Erika",
						"family_name": "Mustermann",
					},
				},
			}, nil
		},
	}

	mockAuthnProvider := managermock.NewAuthnProviderManagerMock(t)
	mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"given_name":     "Erika",
			"family_name":    "Mustermann",
			userAttributeSub: "sub-1",
		}, nil)

	exec := newTestOpenID4VPExecutorWithProvider(t, svc, mockAuthnProvider)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := exec.Execute(openid4vpNodeContext(runtime, map[string]interface{}{
		common.NodePropertyAllowAuthenticationWithoutLocalUser: true,
	}))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecComplete, resp.Status)
	// Runtime attributes from authn provider are stored in RuntimeData
	assert.Equal(t, "sub-1", resp.RuntimeData[userAttributeSub])
	assert.Equal(t, "Erika", resp.RuntimeData["given_name"])
	// AuthUser is not authenticated (no entity reference resolved), so eligible for provisioning
	assert.Equal(t, dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	mockAuthnProvider.AssertExpectations(t)
}

func TestOpenID4VPExecutorPollFailed(t *testing.T) {
	svc := &fakeOpenID4VPService{
		getResult: func(_ context.Context, _ string) (*openid4vp.RequestState, *tidcommon.ServiceError) {
			return &openid4vp.RequestState{
				Status: openid4vp.StatusFailed, FailureReason: "nonce mismatch",
			}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := exec.Execute(openid4vpNodeContext(runtime, nil))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPVerificationFailed.Code, resp.Error.Code)
	assert.Contains(t, resp.Error.ErrorDescription.String(), "nonce mismatch")
}

func TestOpenID4VPExecutorPollExpired(t *testing.T) {
	svc := &fakeOpenID4VPService{
		getResult: func(_ context.Context, _ string) (*openid4vp.RequestState, *tidcommon.ServiceError) {
			return nil, &tidcommon.InternalServerError
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	resp, err := exec.Execute(openid4vpNodeContext(map[string]string{common.RuntimeKeyOpenID4VPState: "gone"}, nil))
	require.NoError(t, err)
	assert.Equal(t, providers.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPExpired.Code, resp.Error.Code)
}
