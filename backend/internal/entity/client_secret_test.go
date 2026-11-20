// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// regenerationFixture is an entity service with a holder for agents, whose system flows and secret write
// are recorded in order.
type regenerationFixture struct {
	svc    *entityService
	holder *ClientSecretHolderInterfaceMock
	flows  *systemFlowProviderMock
	order  []string
	runs   []*flowcommon.SystemFlowContext
	stored json.RawMessage
}

const testSecretEntityID = "agent-1"

// secretRotationInputs is what the flows around the agent's secret rotation are given.
var secretRotationInputs = map[string]string{systemFlowInputEntity: testSecretEntityID}

func (s *ServiceTestSuite) newRegenerationFixture() *regenerationFixture {
	f := &regenerationFixture{svc: s.svc.(*entityService)}
	f.holder = NewClientSecretHolderInterfaceMock(s.T())
	f.svc.RegisterClientSecretHolder(providers.EntityCategoryAgent, f.holder)
	f.flows = newSystemFlowProviderMock(s.T())
	f.svc.systemFlows = f.flows

	e := testEntity(testSecretEntityID)
	s.store.On("GetEntityWithCredentials", mock.Anything, e.ID).
		Return(&entityWithCredentials{Entity: e}, nil).Maybe()
	s.store.On("UpdateSystemCredentials", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Run(func(args mock.Arguments) {
			f.order = append(f.order, "secret")
			f.stored, _ = args.Get(2).(json.RawMessage)
		}).Return(nil).Maybe()
	s.store.On("LockEntity", mock.Anything, e.ID).Return(*e, nil).Maybe()
	s.store.On("UpdateSystemAttributes", mock.Anything, e.ID, mock.IsType(json.RawMessage(nil))).
		Return(nil).Maybe()
	return f
}

// allows has the holder allow the regeneration.
func (f *regenerationFixture) allows() {
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).Return(nil)
}

// recordsFlows records every system flow run, failing the run of the handle named failing with failure.
func (f *regenerationFixture) recordsFlows(failing string, failure *tidcommon.ServiceError) {
	f.flows.EXPECT().ExecuteSystemFlow(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, sysCtx *flowcommon.SystemFlowContext) (*flowcommon.SystemFlowResult,
			*tidcommon.ServiceError) {
			f.runs = append(f.runs, sysCtx)
			f.order = append(f.order, sysCtx.Handle)
			if sysCtx.Handle == failing {
				return nil, failure
			}
			return &flowcommon.SystemFlowResult{RuntimeData: map[string]string{"plan": sysCtx.Handle}}, nil
		})
}

// withTokenRevocation sets whether the server enforces token revocation for the rest of the test.
func (s *ServiceTestSuite) withTokenRevocation(enabled bool) {
	config.ResetServerRuntime()
	cfg := &config.Config{}
	cfg.OAuth.TokenRevocation.Enabled = &enabled
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), cfg))
	s.T().Cleanup(config.ResetServerRuntime)
}

// A regeneration revokes what the old secret issued before the new one is written, and once it is written
// moves the cutoff past the write, with what the first flow shared.
func (s *ServiceTestSuite) TestRegenerateClientSecret_RevokesAroundTheWrite() {
	s.withTokenRevocation(true)
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows("", nil)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.True(regenerated.Revoked)
	s.Nil(regenerated.RevocationError)
	s.Equal([]*flowcommon.SystemFlowContext{
		{Handle: systemFlowSecretRotatePre, Inputs: secretRotationInputs},
		{Handle: systemFlowRevocationPost, Inputs: secretRotationInputs,
			RuntimeData: map[string]string{"plan": systemFlowSecretRotatePre}},
	}, f.runs)
	s.Equal([]string{systemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order,
		"the secret was written before the revocation")
	s.NotContains(string(f.stored), regenerated.ClientSecret, "the secret is stored hashed")
}

// With token revocation off, nothing enforces what the flows record, so the result does not claim a revocation.
func (s *ServiceTestSuite) TestRegenerateClientSecret_WithTokenRevocationOff() {
	s.withTokenRevocation(false)
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows("", nil)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.False(regenerated.Revoked)
	s.Nil(regenerated.RevocationError)
}

// A refusal of the service owning the entity runs no flow and writes nothing.
func (s *ServiceTestSuite) TestRegenerateClientSecret_HolderRefusalWritesNothing() {
	f := s.newRegenerationFixture()
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AGT-1001"}
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).Return(refusal)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(refusal, svcErr)
	s.Nil(regenerated)
	s.Empty(f.order)
}

// A refusal of the flow run before the write writes nothing.
func (s *ServiceTestSuite) TestRegenerateClientSecret_PreFlowRefusalWritesNothing() {
	f := s.newRegenerationFixture()
	f.allows()
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "FET-1088"}
	f.recordsFlows(systemFlowSecretRotatePre, refusal)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(refusal, svcErr)
	s.Nil(regenerated)
	s.Equal([]string{systemFlowSecretRotatePre}, f.order, "something was written after the refusal")
}

// A revocation that fails once the new secret is written still returns the secret, which is stored by then
// and can never be read again, with the failure for the caller to report.
func (s *ServiceTestSuite) TestRegenerateClientSecret_FailedPostFlowStillReturnsTheSecret() {
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows(systemFlowRevocationPost, &tidcommon.InternalServerError)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.Equal(&tidcommon.InternalServerError, regenerated.RevocationError)
	s.False(regenerated.Revoked)
	s.Equal([]string{systemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order)
}

// A secret that cannot be stored is reported as a server error, and no flow runs after the failed write.
func (s *ServiceTestSuite) TestRegenerateClientSecret_FailedWriteRunsNoPostFlow() {
	s.store.On("UpdateSystemCredentials", mock.Anything, testSecretEntityID, mock.IsType(json.RawMessage(nil))).
		Return(s.testErr).Once()
	f := s.newRegenerationFixture()
	f.allows()
	f.recordsFlows("", nil)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(regenerated)
	s.Equal([]string{systemFlowSecretRotatePre}, f.order, "a flow ran after the failed write")
}

// With system flows off, as on a control plane, the secret is still regenerated.
func (s *ServiceTestSuite) TestRegenerateClientSecret_WithSystemFlowsOff() {
	f := s.newRegenerationFixture()
	f.allows()
	f.svc.systemFlows = nil

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Nil(svcErr)
	s.Require().NotNil(regenerated)
	s.NotEmpty(regenerated.ClientSecret)
	s.False(regenerated.Revoked, "nothing was revoked with system flows off")
	s.Equal([]string{"secret"}, f.order)
}

// A category no service decides for is a wiring fault: nothing runs and nothing is written.
func (s *ServiceTestSuite) TestRegenerateClientSecret_CategoryWithoutAHolder() {
	f := s.newRegenerationFixture()

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryUser, testSecretEntityID)

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(regenerated)
	s.Empty(f.order)
}

// A rotation whose write fails runs no flow after it, and reports the write's failure.
func (s *ServiceTestSuite) TestRotateClientSecret_FailedWriteRunsNoPostFlow() {
	f := s.newRegenerationFixture()
	f.recordsFlows("", nil)

	svcErr, revocationErr := f.svc.RotateClientSecret(s.ctx, testSecretEntityID, func() *tidcommon.ServiceError {
		f.order = append(f.order, "write")
		return &tidcommon.InternalServerError
	})

	s.Equal(&tidcommon.InternalServerError, svcErr)
	s.Nil(revocationErr)
	s.Equal([]string{systemFlowSecretRotatePre, "write"}, f.order)
}

// The server injects the flow engine once it exists; a service that cannot take it is reported.
func (s *ServiceTestSuite) TestSetSystemFlowProvider() {
	service := &entityService{}
	provider := newSystemFlowProviderMock(s.T())

	s.NoError(SetSystemFlowProvider(service, provider))
	s.Same(provider, service.systemFlows)
	s.Error(SetSystemFlowProvider(NewEntityServiceInterfaceMock(s.T()), provider))
	s.Error(SetSystemFlowProvider(nil, provider))
}
