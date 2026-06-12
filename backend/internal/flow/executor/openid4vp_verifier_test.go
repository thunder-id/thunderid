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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type fakeOpenID4VPService struct {
	initiate func(ctx context.Context, definitionID string) (*openid4vp.Initiation, error)
	result   func(ctx context.Context, state string) (*openid4vp.RequestState, error)
}

func (f *fakeOpenID4VPService) Initiate(ctx context.Context, definitionID string) (*openid4vp.Initiation, error) {
	return f.initiate(ctx, definitionID)
}

func (f *fakeOpenID4VPService) Result(ctx context.Context, state string) (*openid4vp.RequestState, error) {
	return f.result(ctx, state)
}

func newTestOpenID4VPExecutor(t *testing.T, service openid4vpVerifierService) core.ExecutorInterface {
	t.Helper()
	return newTestOpenID4VPExecutorWithProvider(t, service, nil)
}

func newTestOpenID4VPExecutorWithProvider(t *testing.T, service openid4vpVerifierService,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface) core.ExecutorInterface {
	t.Helper()
	factory := coremock.NewFlowFactoryInterfaceMock(t)
	base := coremock.NewExecutorInterfaceMock(t)
	factory.On("CreateExecutor", ExecutorNameOpenID4VPVerify, common.ExecutorTypeAuthentication,
		[]common.Input{}, []common.Input{}).Return(base).Maybe()
	return newOpenID4VPVerifier(factory, service, nil, authnProvider)
}

func openid4vpNodeContext(runtime map[string]string, properties map[string]interface{}) *core.NodeContext {
	if runtime == nil {
		runtime = map[string]string{}
	}
	return &core.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "exec-1",
		RuntimeData:    runtime,
		NodeProperties: properties,
	}
}

func TestOpenID4VPExecutorInitiates(t *testing.T) {
	var seenDefID string
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, defID string) (*openid4vp.Initiation, error) {
			seenDefID = defID
			return &openid4vp.Initiation{
				State:      "state-123",
				ClientID:   "x509_hash:abc",
				RequestURI: "https://verifier.example/openid4vp/request?state=state-123",
			}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	props := map[string]interface{}{propertyKeyPresentationDefinitionID: "custom-def"}
	resp, err := exec.Execute(openid4vpNodeContext(nil, props))
	require.NoError(t, err)
	assert.Equal(t, "custom-def", seenDefID, "executor must pass the configured definition id")
	assert.Equal(t, common.ExecUserInputRequired, resp.Status)
	assert.Equal(t, "state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	assert.Equal(t, "x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

// When no presentation_definition_id is configured on the node, the executor
// falls back to the EUDI PID id for Phase 1 back-compat.
func TestOpenID4VPExecutorDefaultsToEUDIPID(t *testing.T) {
	var seenDefID string
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, defID string) (*openid4vp.Initiation, error) {
			seenDefID = defID
			return &openid4vp.Initiation{State: "s", ClientID: "x509_hash:abc", RequestURI: "https://x"}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	_, err := exec.Execute(openid4vpNodeContext(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, defaultPresentationDefinitionID, seenDefID)
}

func TestOpenID4VPExecutorInitiateFailure(t *testing.T) {
	svc := &fakeOpenID4VPService{
		initiate: func(_ context.Context, _ string) (*openid4vp.Initiation, error) {
			return nil, errors.New("boom")
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	resp, err := exec.Execute(openid4vpNodeContext(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPInitiateFailed.Code, resp.Error.Code)
}

func TestOpenID4VPExecutorPollPending(t *testing.T) {
	svc := &fakeOpenID4VPService{
		result: func(_ context.Context, state string) (*openid4vp.RequestState, error) {
			return &openid4vp.RequestState{
				State:      state,
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
	assert.Equal(t, common.ExecUserInputRequired, resp.Status)
	assert.Equal(t, "state-123", resp.RuntimeData[common.RuntimeKeyOpenID4VPState])
	// QR data must persist across polls so the wait view keeps rendering it.
	assert.Equal(t, "x509_hash:abc", resp.AdditionalData[common.DataOpenID4VPClientID])
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPRequestURI], "state-123")
	assert.Contains(t, resp.AdditionalData[common.DataOpenID4VPWalletURI], "openid4vp://")
}

func TestOpenID4VPExecutorPollCompleted(t *testing.T) {
	svc := &fakeOpenID4VPService{
		result: func(_ context.Context, state string) (*openid4vp.RequestState, error) {
			return &openid4vp.RequestState{
				State:  state,
				Status: openid4vp.StatusCompleted,
				Result: &openid4vp.VerifiedPresentation{
					Subject: "sub-1",
					Issuer:  "https://issuer.example",
					VCT:     "urn:eudi:pid:de:1",
					Claims:  map[string]interface{}{"given_name": "Erika", "family_name": "Mustermann"},
				},
			}, nil
		},
	}

	mockAuthnProvider := managermock.NewAuthnProviderManagerInterfaceMock(t)
	mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, authnprovidercm.AuthenticatedClaims{
			"given_name":       "Erika",
			"family_name":      "Mustermann",
			"openid4vp_issuer": "https://issuer.example",
			"openid4vp_vct":    "urn:eudi:pid:de:1",
			userAttributeSub:   "sub-1",
		}, nil)

	exec := newTestOpenID4VPExecutorWithProvider(t, svc, mockAuthnProvider)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := exec.Execute(openid4vpNodeContext(runtime, nil))
	require.NoError(t, err)
	assert.Equal(t, common.ExecComplete, resp.Status)
	// Runtime attributes from authn provider are stored in RuntimeData
	assert.Equal(t, "sub-1", resp.RuntimeData[userAttributeSub])
	assert.Equal(t, "Erika", resp.RuntimeData["given_name"])
	// AuthUser is not authenticated (no entity reference resolved), so eligible for provisioning
	assert.Equal(t, dataValueTrue, resp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	mockAuthnProvider.AssertExpectations(t)
}

func TestOpenID4VPExecutorPollFailed(t *testing.T) {
	svc := &fakeOpenID4VPService{
		result: func(_ context.Context, state string) (*openid4vp.RequestState, error) {
			return &openid4vp.RequestState{
				State: state, Status: openid4vp.StatusFailed, FailureReason: "nonce mismatch",
			}, nil
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	runtime := map[string]string{common.RuntimeKeyOpenID4VPState: "state-123"}
	resp, err := exec.Execute(openid4vpNodeContext(runtime, nil))
	require.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPVerificationFailed.Code, resp.Error.Code)
	assert.Contains(t, resp.Error.ErrorDescription.DefaultValue, "nonce mismatch")
}

func TestOpenID4VPExecutorPollExpired(t *testing.T) {
	svc := &fakeOpenID4VPService{
		result: func(_ context.Context, _ string) (*openid4vp.RequestState, error) {
			return nil, openid4vp.ErrUnknownState
		},
	}
	exec := newTestOpenID4VPExecutor(t, svc)

	resp, err := exec.Execute(openid4vpNodeContext(map[string]string{common.RuntimeKeyOpenID4VPState: "gone"}, nil))
	require.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPExpired.Code, resp.Error.Code)
}

func TestOpenID4VPExecutorNotConfigured(t *testing.T) {
	exec := newTestOpenID4VPExecutor(t, nil)
	resp, err := exec.Execute(openid4vpNodeContext(nil, nil))
	require.NoError(t, err)
	assert.Equal(t, common.ExecFailure, resp.Status)
	assert.Equal(t, ErrOpenID4VPNotConfigured.Code, resp.Error.Code)
}
