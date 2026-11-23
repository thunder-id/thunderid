// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit/audittest"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

const (
	testAuditActorID = "operator"
	testAuditEmail   = "private.contact@example.com"
	testAuditMobile  = "+94771234567"
)

// bothVerified is stored verification metadata with workEmail and mobile verified.
const bothVerified = `{"attributeVerification":{"workEmail":{"verifiedAt":"2026-10-15T00:00:00Z"},` +
	`"mobile":{"verifiedAt":"2026-10-15T00:00:00Z"}}}`

// VerificationAuditTestSuite checks the audit event each verification a committed write clears publishes.
type VerificationAuditTestSuite struct {
	suite.Suite
}

func TestVerificationAuditTestSuite(t *testing.T) {
	suite.Run(t, new(VerificationAuditTestSuite))
}

// clearingService returns a service whose update of user-1's attributes to an empty set clears the
// verification of workEmail and mobile, publishing on bus.
func (s *VerificationAuditTestSuite) clearingService(bus eventbus.EventBusInterface) *entityService {
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	expectAttributeUpdate(store, verificationUser(
		`{"workEmail":"`+testAuditEmail+`","mobile":"`+testAuditMobile+`"}`, bothVerified))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).Return(nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	return &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: markingTransactioner(s.T()), bus: bus, logger: log.GetLogger()}
}

func (s *VerificationAuditTestSuite) TestAClearedVerificationPublishesOneEventEach() {
	bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
	svc := s.clearingService(bus)
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou1", "", nil, nil))

	s.Require().NoError(svc.UpdateAttributes(ctx, "user-1", json.RawMessage(`{}`)))

	published := events()
	s.Require().Len(published, 2)
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "user-1"}
	attributes := make([]string, 0, len(published))
	for _, evt := range published {
		s.Equal(audit.EventUserVerificationInvalidate, evt.Name)
		s.Equal(user, evt.Payload.Target())
		s.Equal(user, evt.Payload.Subject())
		s.Equal(audit.ActionInvalidate, evt.Payload.Action())
		s.Equal(audit.OutcomeSucceeded, evt.Payload.Outcome())
		s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}, evt.Payload.Actor())
		s.Require().Len(evt.Payload.Details(), 1)
		attributes = append(attributes, evt.Payload.Details()[audit.DetailAttribute])
		recorded := fmt.Sprintf("%+v", evt.Payload)
		s.NotContains(recorded, testAuditEmail, "a contact is never recorded")
		s.NotContains(recorded, testAuditMobile, "a contact is never recorded")
		s.NotContains(recorded, string(verificationValueRemoved), "the invalidation reason is not recorded")
	}
	s.ElementsMatch([]string{"workEmail", "mobile"}, attributes)
}

func (s *VerificationAuditTestSuite) TestARuntimeContextIsAnonymous() {
	signedIn := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou1", "", nil, nil))
	for name, ctx := range map[string]context.Context{
		"no caller":          security.WithRuntimeContext(context.Background()),
		"a caller in a flow": security.WithRuntimeContext(signedIn),
	} {
		s.Run(name, func() {
			bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
			svc := s.clearingService(bus)

			s.Require().NoError(svc.UpdateAttributes(ctx, "user-1", json.RawMessage(`{}`)))

			published := events()
			s.Require().Len(published, 2)
			for _, evt := range published {
				s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, evt.Payload.Actor())
			}
		})
	}
}

func (s *VerificationAuditTestSuite) TestAuditDoesNotNeedObservability() {
	bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
	svc := s.clearingService(bus)
	svc.observability = nil

	s.Require().NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))

	s.Len(events(), 2)
}

func (s *VerificationAuditTestSuite) TestAWriteThatClearsNothingPublishesNoEvent() {
	bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	expectAttributeUpdate(store, verificationUser(`{"workEmail":"`+testAuditEmail+`"}`, `{}`))
	store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	svc := &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: markingTransactioner(s.T()), bus: bus, logger: log.GetLogger()}

	s.Require().NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))

	s.Empty(events())
}

// A verification is cleared only once its write commits, so a transaction that fails, in its writes or at
// its commit, publishes nothing.
func (s *VerificationAuditTestSuite) TestAWriteThatDoesNotCommitPublishesNoEvent() {
	cases := map[string]struct {
		systemWrite error
		commit      error
	}{
		"a write in the transaction fails": {systemWrite: errors.New("write failed")},
		"the commit fails":                 {commit: errors.New("commit failed")},
	}
	for name, tc := range cases {
		s.Run(name, func() {
			bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
			store := newEntityStoreInterfaceMock(s.T())
			store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
			expectAttributeUpdate(store, verificationUser(
				`{"workEmail":"`+testAuditEmail+`","mobile":"`+testAuditMobile+`"}`, bothVerified))
			store.On("UpdateAttributes", inTransaction(), "user-1", mock.Anything).Return(nil).Once()
			store.On("UpdateSystemAttributes", inTransaction(), "user-1", withoutVerification()).
				Return(tc.systemWrite).Once()
			runtime := newEntityRuntimeStoreInterfaceMock(s.T())
			installRuntimeFixture(runtime)
			transactioner := transactionmock.NewTransactionerMock(s.T())
			transactioner.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
				func(ctx context.Context, txFunc func(context.Context) error) error {
					if err := txFunc(context.WithValue(ctx, txKey{}, true)); err != nil {
						return err
					}
					return tc.commit
				}).Once()
			svc := &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
				transactioner: transactioner, bus: bus, logger: log.GetLogger()}

			err := svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`))

			s.Require().Error(err)
			s.Empty(events())
		})
	}
}

// UpdateEntity clears a verification through the same event as UpdateAttributes.
func (s *VerificationAuditTestSuite) TestUpdateEntityPublishesTheClearedVerification() {
	bus, events := audittest.NewRecordingBus(s.T(), "entity-service-test")
	current := verificationUser(`{"workEmail":"`+testAuditEmail+`"}`, verifiedWorkEmail)
	store := newEntityStoreInterfaceMock(s.T())
	store.On("GetIndexedAttributes").Return(map[string]bool{}).Maybe()
	store.On("LockEntity", inTransaction(), "user-1").Return(current, nil).Once()
	store.On("UpdateEntity", inTransaction(), mock.Anything).Return(nil).Once()
	store.On("GetEntity", inTransaction(), "user-1").Return(current, nil).Once()
	runtime := newEntityRuntimeStoreInterfaceMock(s.T())
	installRuntimeFixture(runtime)
	svc := &entityService{store: store, runtimeStore: runtime, entityTypeService: contactTypeService(s.T()),
		transactioner: markingTransactioner(s.T()), bus: bus, logger: log.GetLogger()}
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditActorID, "ou1", "", nil, nil))
	next := current
	next.Attributes = json.RawMessage(`{}`)

	_, err := svc.UpdateEntity(ctx, "user-1", &next)

	s.Require().NoError(err)
	published := events()
	s.Require().Len(published, 1)
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "user-1"}
	s.Equal(audit.EventUserVerificationInvalidate, published[0].Name)
	s.Equal(user, published[0].Payload.Target())
	s.Equal(map[string]string{audit.DetailAttribute: "workEmail"}, published[0].Payload.Details())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditActorID}, published[0].Payload.Actor())
}

func (s *VerificationAuditTestSuite) TestAFailedPublishDoesNotFailTheWrite() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(fmt.Errorf("audit queue is full")).Twice()
	svc := s.clearingService(failing)

	s.NoError(svc.UpdateAttributes(context.Background(), "user-1", json.RawMessage(`{}`)))
}

func (s *VerificationAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(nil, nil, nil, nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
