// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditName        = "PAYMENTS_API_TOKEN"
	testAuditValue       = "audit-test-value-0123456789abcdef"
	testAuditDescription = "Token the payments connector presents"
)

// publishedEvent is one audit event the variable store published.
type publishedEvent struct {
	name    string
	payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, publishedEvent{name: env.Name, payload: payload})
	return nil
}

// newRecordingBus returns an event bus with a sink subscribed the way the binaries subscribe theirs, so
// an event audit refuses never reaches it, and a function that stops the bus and returns the audit
// events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "variable-store-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// newDiscardingBus returns an event bus that accepts and drops every event, for a test the audit event
// does not matter to. A test of what the service publishes uses newRecordingBus.
func newDiscardingBus(t *testing.T) eventbus.EventBusInterface {
	bus := eventbusmock.NewEventBusInterfaceMock(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return bus
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// entry is what a test sends to create or update a variable or a secret.
type entry struct {
	name, value, description string
}

// validEntry is an entry the service accepts, called name.
func validEntry(name string) entry {
	return entry{name: name, value: testAuditValue, description: testAuditDescription}
}

// storeKind is one of the two collections the store serves, with the calls a test makes on it.
type storeKind struct {
	name       string
	targetType string
	create     string
	update     string
	delete     string
	// expectRead arranges the read by name the service makes before a create and after a write.
	expectRead func(m *storeInterfaceMock, held bool)
	// expectReadFails arranges the read by name after a write to fail with err.
	expectReadFails func(m *storeInterfaceMock, err error)
	// expectInsert arranges the insert of a create, returning inserted and err.
	expectInsert func(m *storeInterfaceMock, inserted bool, err error)
	// expectUpsert arranges the write of an update, returning created and err.
	expectUpsert func(m *storeInterfaceMock, created bool, err error)
	// expectDelete arranges the delete, returning deleted and err as the real store does.
	expectDelete func(m *storeInterfaceMock, deleted bool, err error)
	callCreate   func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError
	callUpdate   func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError
	callDelete   func(s ServiceInterface, ctx context.Context, name string) *common.ServiceError
}

func storeKinds() []storeKind {
	return []storeKind{
		{
			name: "variable", targetType: audit.TargetTypeVariable,
			create: audit.EventVariableCreate, update: audit.EventVariableUpdate, delete: audit.EventVariableDelete,
			expectRead: func(m *storeInterfaceMock, held bool) {
				var variable *Variable
				if held {
					variable = &Variable{Name: testAuditName, Value: testAuditValue, Description: testAuditDescription}
				}
				m.EXPECT().GetVariable(mock.Anything, testAuditName).Return(variable, nil).Once()
			},
			expectReadFails: func(m *storeInterfaceMock, err error) {
				m.EXPECT().GetVariable(mock.Anything, testAuditName).Return(nil, err).Once()
			},
			expectInsert: func(m *storeInterfaceMock, inserted bool, err error) {
				m.EXPECT().InsertVariable(mock.Anything, mock.Anything).Return(inserted, err).Once()
			},
			expectUpsert: func(m *storeInterfaceMock, created bool, err error) {
				m.EXPECT().UpsertVariable(mock.Anything, mock.Anything).Return(created, err).Once()
			},
			expectDelete: func(m *storeInterfaceMock, deleted bool, err error) {
				m.EXPECT().DeleteVariable(mock.Anything, testAuditName).Return(deleted, err).Once()
			},
			callCreate: func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError {
				_, svcErr := s.CreateVariable(ctx, VariableRequest{
					Name: e.name, Value: e.value, Description: e.description})
				return svcErr
			},
			callUpdate: func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError {
				_, _, svcErr := s.UpdateVariable(ctx, e.name,
					VariableUpdateRequest{Value: e.value, Description: e.description})
				return svcErr
			},
			callDelete: func(s ServiceInterface, ctx context.Context, name string) *common.ServiceError {
				return s.DeleteVariable(ctx, name)
			},
		},
		{
			name: "secret", targetType: audit.TargetTypeSecret,
			create: audit.EventSecretCreate, update: audit.EventSecretUpdate, delete: audit.EventSecretDelete,
			expectRead: func(m *storeInterfaceMock, held bool) {
				var secret *Secret
				if held {
					secret = &Secret{Name: testAuditName, Exists: true, Description: testAuditDescription}
				}
				m.EXPECT().GetSecret(mock.Anything, testAuditName).Return(secret, nil).Once()
			},
			expectReadFails: func(m *storeInterfaceMock, err error) {
				m.EXPECT().GetSecret(mock.Anything, testAuditName).Return(nil, err).Once()
			},
			expectInsert: func(m *storeInterfaceMock, inserted bool, err error) {
				m.EXPECT().InsertSecret(mock.Anything, testAuditName, mock.Anything, testAuditDescription).
					Return(inserted, err).Once()
			},
			expectUpsert: func(m *storeInterfaceMock, created bool, err error) {
				m.EXPECT().UpsertSecret(mock.Anything, testAuditName, mock.Anything, testAuditDescription).
					Return(created, err).Once()
			},
			expectDelete: func(m *storeInterfaceMock, deleted bool, err error) {
				m.EXPECT().DeleteSecret(mock.Anything, testAuditName).Return(deleted, err).Once()
			},
			callCreate: func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError {
				_, svcErr := s.CreateSecret(ctx, SecretRequest{
					Name: e.name, Value: e.value, Description: e.description})
				return svcErr
			},
			callUpdate: func(s ServiceInterface, ctx context.Context, e entry) *common.ServiceError {
				_, _, svcErr := s.UpdateSecret(ctx, e.name,
					SecretUpdateRequest{Value: e.value, Description: e.description})
				return svcErr
			},
			callDelete: func(s ServiceInterface, ctx context.Context, name string) *common.ServiceError {
				return s.DeleteSecret(ctx, name)
			},
		},
	}
}

type ServiceAuditTestSuite struct {
	suite.Suite
	store *storeInterfaceMock
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	s.store = newStoreInterfaceMock(s.T())
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.store = newStoreInterfaceMock(s.T())
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the entry called name. An attempt that succeeded has the entry as subject; a failed one has none.
func (s *ServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, event, targetType string,
	action audit.Action, name string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	published := events[0]
	target := audit.Ref{Type: targetType, ID: name}
	s.Equal(event, published.name)
	s.Equal(target, published.payload.Target())
	s.Equal(action, published.payload.Action())
	s.Equal(outcome, published.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, published.payload.Actor())
	s.Empty(published.payload.Details())
	if outcome.Result == audit.ResultFailed {
		s.Equal(audit.Ref{}, published.payload.Subject(), "a failed attempt may name nothing stored")
	} else {
		s.Equal(target, published.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", published.payload)
	s.NotContains(recorded, testAuditValue, "a value is never recorded")
	s.NotContains(recorded, testAuditDescription, "a description is never recorded")
}

func (s *ServiceAuditTestSuite) TestCreate_PublishesTheCreatedEntry() {
	for _, kind := range storeKinds() {
		s.Run(kind.name, func() {
			bus, published := newRecordingBus(s.T())
			kind.expectRead(s.store, false)
			kind.expectInsert(s.store, true, nil)
			kind.expectRead(s.store, true)

			svcErr := kind.callCreate(newService(s.store, reversingCrypto{}, bus), adminContext(),
				validEntry(testAuditName))

			s.Require().Nil(svcErr)
			s.requireOnlyEvent(published(), kind.create, kind.targetType, audit.ActionCreate, testAuditName,
				audit.OutcomeSucceeded)
		})
	}
}

func (s *ServiceAuditTestSuite) TestCreate_PublishesAFailure() {
	tooLong := strings.Repeat("v", maxValueLength+1)
	for _, kind := range storeKinds() {
		cases := []struct {
			name string
			// only limits the case to the kind of that name when set.
			only    string
			entry   entry
			crypto  reversingCrypto
			arrange func()
			outcome audit.Outcome
		}{
			{name: "the name is invalid", entry: validEntry("1-not-a-name"), arrange: func() {},
				outcome: audit.OutcomeFailed("var_1002")},
			{name: "the value is too long", entry: entry{testAuditName, tooLong, testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1003")},
			{name: "the value is a reference", entry: entry{testAuditName, "sec:OTHER_SECRET", testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1011")},
			{name: "the description is too long",
				entry:   entry{testAuditName, testAuditValue, strings.Repeat("d", maxDescriptionLength+1)},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1008")},
			{name: "the secret is empty", only: "secret", entry: entry{testAuditName, "", testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1003")},
			{name: "the name is taken", entry: validEntry(testAuditName),
				arrange: func() { kind.expectRead(s.store, true) }, outcome: audit.OutcomeFailed("var_1009")},
			{name: "a concurrent create took the name", entry: validEntry(testAuditName), arrange: func() {
				kind.expectRead(s.store, false)
				kind.expectInsert(s.store, false, nil)
			}, outcome: audit.OutcomeFailed("var_1009")},
			{name: "sealing the secret fails", only: "secret", entry: validEntry(testAuditName),
				crypto:  reversingCrypto{err: errors.New("key unavailable")},
				arrange: func() { kind.expectRead(s.store, false) }, outcome: audit.OutcomeFailed("var_5000")},
			{name: "the store fails", entry: validEntry(testAuditName), arrange: func() {
				kind.expectRead(s.store, false)
				kind.expectInsert(s.store, false, errors.New("deadlock detected"))
			}, outcome: audit.OutcomeFailed("var_5000")},
		}
		for _, tc := range cases {
			if tc.only != "" && tc.only != kind.name {
				continue
			}
			s.Run(kind.name+": "+tc.name, func() {
				bus, published := newRecordingBus(s.T())
				tc.arrange()

				svcErr := kind.callCreate(newService(s.store, tc.crypto, bus), adminContext(), tc.entry)

				s.Require().NotNil(svcErr)
				s.requireOnlyEvent(published(), kind.create, kind.targetType, audit.ActionCreate, tc.entry.name,
					tc.outcome)
			})
		}
	}
}

// An update stores the entry whether or not the name was in use, and records one update either way.
func (s *ServiceAuditTestSuite) TestUpdate_PublishesOneUpdatePerAttempt() {
	for _, kind := range storeKinds() {
		cases := []struct {
			name string
			// only limits the case to the kind of that name when set.
			only    string
			entry   entry
			crypto  reversingCrypto
			arrange func()
			outcome audit.Outcome
		}{
			{name: "the entry is replaced", entry: validEntry(testAuditName), arrange: func() {
				kind.expectUpsert(s.store, false, nil)
				kind.expectRead(s.store, true)
			}, outcome: audit.OutcomeSucceeded},
			{name: "the name was unused, so the entry is created", entry: validEntry(testAuditName), arrange: func() {
				kind.expectUpsert(s.store, true, nil)
				kind.expectRead(s.store, true)
			}, outcome: audit.OutcomeSucceeded},
			{name: "the value is a reference", entry: entry{testAuditName, "sec:OTHER_SECRET", testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1011")},
			{name: "the value is too long",
				entry:   entry{testAuditName, strings.Repeat("v", maxValueLength+1), testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1003")},
			{name: "the description is too long",
				entry:   entry{testAuditName, testAuditValue, strings.Repeat("d", maxDescriptionLength+1)},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1008")},
			{name: "the secret is empty", only: "secret", entry: entry{testAuditName, "", testAuditDescription},
				arrange: func() {}, outcome: audit.OutcomeFailed("var_1003")},
			{name: "sealing the secret fails", only: "secret", entry: validEntry(testAuditName),
				crypto: reversingCrypto{err: errors.New("key unavailable")}, arrange: func() {},
				outcome: audit.OutcomeFailed("var_5000")},
			{name: "the store fails", entry: validEntry(testAuditName), arrange: func() {
				kind.expectUpsert(s.store, false, errors.New("deadlock detected"))
			}, outcome: audit.OutcomeFailed("var_5000")},
		}
		for _, tc := range cases {
			if tc.only != "" && tc.only != kind.name {
				continue
			}
			s.Run(kind.name+": "+tc.name, func() {
				bus, published := newRecordingBus(s.T())
				tc.arrange()

				svcErr := kind.callUpdate(newService(s.store, tc.crypto, bus), adminContext(), tc.entry)

				s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
				s.requireOnlyEvent(published(), kind.update, kind.targetType, audit.ActionUpdate, testAuditName,
					tc.outcome)
			})
		}
	}
}

// The event records the write. A read of the entry back, which only builds the response, that fails or
// finds the entry already deleted does not turn a stored entry into a failed attempt in the log.
func (s *ServiceAuditTestSuite) TestAReadBackAfterASuccessfulWriteDoesNotChangeTheOutcome() {
	for _, kind := range storeKinds() {
		cases := []struct {
			name    string
			arrange func()
			errCode string
		}{
			{"the read fails", func() { kind.expectReadFails(s.store, errors.New("connection reset")) },
				ErrorInternalServerError.Code},
			{"a concurrent delete removed the entry", func() { kind.expectRead(s.store, false) },
				ErrorNotFound.Code},
		}
		for _, tc := range cases {
			s.Run(kind.name+": create, "+tc.name, func() {
				bus, published := newRecordingBus(s.T())
				kind.expectRead(s.store, false)
				kind.expectInsert(s.store, true, nil)
				tc.arrange()

				svcErr := kind.callCreate(newService(s.store, reversingCrypto{}, bus), adminContext(),
					validEntry(testAuditName))

				s.Require().NotNil(svcErr)
				s.Equal(tc.errCode, svcErr.Code)
				s.requireOnlyEvent(published(), kind.create, kind.targetType, audit.ActionCreate, testAuditName,
					audit.OutcomeSucceeded)
			})
			s.Run(kind.name+": update, "+tc.name, func() {
				bus, published := newRecordingBus(s.T())
				kind.expectUpsert(s.store, false, nil)
				tc.arrange()

				svcErr := kind.callUpdate(newService(s.store, reversingCrypto{}, bus), adminContext(),
					validEntry(testAuditName))

				s.Require().NotNil(svcErr)
				s.Equal(tc.errCode, svcErr.Code)
				s.requireOnlyEvent(published(), kind.update, kind.targetType, audit.ActionUpdate, testAuditName,
					audit.OutcomeSucceeded)
			})
		}
	}
}

// A failed create or update keeps the name the caller sent, cut to the length the payload records.
func (s *ServiceAuditTestSuite) TestAnInvalidNameIsRecordedCapped() {
	probed := strings.Repeat("A", 300)
	for _, kind := range storeKinds() {
		s.Run(kind.name+": create", func() {
			bus, published := newRecordingBus(s.T())

			svcErr := kind.callCreate(newService(s.store, reversingCrypto{}, bus), adminContext(), validEntry(probed))

			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), kind.create, kind.targetType, audit.ActionCreate,
				strings.Repeat("A", 128), audit.OutcomeFailed("var_1002"))
		})
		s.Run(kind.name+": update", func() {
			bus, published := newRecordingBus(s.T())

			svcErr := kind.callUpdate(newService(s.store, reversingCrypto{}, bus), adminContext(),
				validEntry(probed))

			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), kind.update, kind.targetType, audit.ActionUpdate,
				strings.Repeat("A", 128), audit.OutcomeFailed("var_1002"))
		})
	}
}

func (s *ServiceAuditTestSuite) TestDelete_PublishesOneEventPerAttempt() {
	for _, kind := range storeKinds() {
		cases := []struct {
			name      string
			entryName string
			arrange   func()
			outcome   audit.Outcome
		}{
			{"the entry is deleted", testAuditName, func() { kind.expectDelete(s.store, true, nil) },
				audit.OutcomeSucceeded},
			{"the name is invalid", "1-not-a-name", func() {}, audit.OutcomeFailed("var_1002")},
			{"the store fails", testAuditName,
				func() { kind.expectDelete(s.store, false, errors.New("deadlock detected")) },
				audit.OutcomeFailed("var_5000")},
		}
		for _, tc := range cases {
			s.Run(kind.name+": "+tc.name, func() {
				bus, published := newRecordingBus(s.T())
				tc.arrange()

				svcErr := kind.callDelete(newService(s.store, reversingCrypto{}, bus), adminContext(), tc.entryName)

				s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
				s.requireOnlyEvent(published(), kind.delete, kind.targetType, audit.ActionDelete, tc.entryName,
					tc.outcome)
			})
		}
	}
}

// A delete stays idempotent, but one that matched nothing records nothing, so a caller cannot write a
// succeeded delete for any name it chooses. The store reports the rows the delete matched.
func (s *ServiceAuditTestSuite) TestDelete_PublishesNothingForAMissingName() {
	for _, kind := range storeKinds() {
		s.Run(kind.name, func() {
			bus, published := newRecordingBus(s.T())
			kind.expectDelete(s.store, false, nil)

			svcErr := kind.callDelete(newService(s.store, reversingCrypto{}, bus), adminContext(), testAuditName)

			s.Nil(svcErr)
			s.Empty(published())
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceAuditTestSuite) TestAFailingBusDoesNotFailTheChange() {
	for _, kind := range storeKinds() {
		s.Run(kind.name, func() {
			failing := eventbusmock.NewEventBusInterfaceMock(s.T())
			failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Times(3)
			service := newService(s.store, reversingCrypto{}, failing)
			kind.expectRead(s.store, false)
			kind.expectInsert(s.store, true, nil)
			kind.expectRead(s.store, true)
			kind.expectUpsert(s.store, false, nil)
			kind.expectRead(s.store, true)
			kind.expectDelete(s.store, true, nil)

			s.Nil(kind.callCreate(service, adminContext(), validEntry(testAuditName)))
			s.Nil(kind.callUpdate(service, adminContext(), validEntry(testAuditName)))
			s.Nil(kind.callDelete(service, adminContext(), testAuditName))
		})
	}
}
