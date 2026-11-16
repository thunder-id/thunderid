// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"

	"github.com/stretchr/testify/mock"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// regenerationFixture is an entity service with a holder for agents, whose system flows and secret write
// are recorded in order.
type regenerationFixture struct {
	svc    *entityService
	holder *ClientSecretHolderInterfaceMock
	flows  *SystemFlowProviderInterfaceMock
	order  []string
	runs   []*flowcommon.SystemFlowContext
	stored json.RawMessage
}

const testSecretEntityID = "agent-1"

// secretRotationInputs is what the flows around the agent's secret rotation are given.
var secretRotationInputs = map[string]string{
	SystemFlowInputEntity:         testSecretEntityID,
	SystemFlowInputEntityCategory: string(providers.EntityCategoryAgent),
}

func (s *ServiceTestSuite) newRegenerationFixture() *regenerationFixture {
	f := &regenerationFixture{svc: s.svc.(*entityService)}
	f.holder = NewClientSecretHolderInterfaceMock(s.T())
	f.svc.RegisterClientSecretHolder(providers.EntityCategoryAgent, f.holder)
	f.flows = NewSystemFlowProviderInterfaceMock(s.T())
	f.svc.SetSystemFlowProvider(f.flows)

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
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).
		Return(&ClientSecretProfile{ClientID: "client-1"}, nil)
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

// A regeneration revokes what the old secret issued before the new one is written, and once it is written
// moves the cutoff past the write, with what the first flow shared.
func (s *ServiceTestSuite) TestRegenerateClientSecret_RevokesAroundTheWrite() {
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
		{Handle: SystemFlowSecretRotatePre, Inputs: secretRotationInputs},
		{Handle: systemFlowRevocationPost, Inputs: secretRotationInputs,
			RuntimeData: map[string]string{"plan": SystemFlowSecretRotatePre}},
	}, f.runs)
	s.Equal([]string{SystemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order,
		"the secret was written before the revocation")
	s.NotContains(string(f.stored), regenerated.ClientSecret, "the secret is stored hashed")
}

// A refusal of the service owning the entity runs no flow and writes nothing.
func (s *ServiceTestSuite) TestRegenerateClientSecret_HolderRefusalWritesNothing() {
	f := s.newRegenerationFixture()
	refusal := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "AGT-1001"}
	f.holder.EXPECT().ValidateClientSecretRegeneration(mock.Anything, testSecretEntityID).Return(nil, refusal)

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
	f.recordsFlows(SystemFlowSecretRotatePre, refusal)

	regenerated, svcErr := f.svc.RegenerateClientSecret(s.ctx, providers.EntityCategoryAgent, testSecretEntityID)

	s.Equal(refusal, svcErr)
	s.Nil(regenerated)
	s.Equal([]string{SystemFlowSecretRotatePre}, f.order, "something was written after the refusal")
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
	s.Equal([]string{SystemFlowSecretRotatePre, "secret", systemFlowRevocationPost}, f.order)
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

// Validation asks the service registered for the category, and returns its profile.
func (s *ServiceTestSuite) TestValidateClientSecretRegeneration_AsksTheCategorysHolder() {
	f := s.newRegenerationFixture()
	f.allows()

	profile, svcErr := f.svc.ValidateClientSecretRegeneration(s.ctx, providers.EntityCategoryAgent,
		testSecretEntityID)

	s.Nil(svcErr)
	s.Equal(&ClientSecretProfile{ClientID: "client-1"}, profile)
}
