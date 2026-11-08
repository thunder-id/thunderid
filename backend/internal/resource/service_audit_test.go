// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	flowcommon "github.com/thunder-id/thunderid/internal/flow/common"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditServerID     = "019a0000-0000-7000-8000-00000000d001"
	testAuditResourceID   = "019a0000-0000-7000-8000-00000000d002"
	testAuditActionID     = "019a0000-0000-7000-8000-00000000d003"
	testAuditServerName   = "Audited Orders API"
	testAuditIdentifier   = "https://audited-orders.example.com/api"
	testAuditResourceName = "Audited Invoices"
	testAuditHandle       = "auditedinvoices"
	testAuditActionName   = "Audited Approve"
	testAuditActionHandle = "auditedapprove"
)

// publishedEvent is one audit event the resource service published.
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
	require.NoError(t, audit.Subscribe(registry, "resource-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the service publishes uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// ResourceServiceAuditTestSuite covers the audit events the resource service publishes for the resource
// servers, resources and actions it creates, updates and deletes.
type ResourceServiceAuditTestSuite struct {
	suite.Suite
	store   *resourceStoreInterfaceMock
	ou      *oumock.OrganizationUnitServiceInterfaceMock
	project *projectOUValidatorMock
}

func TestResourceServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServiceAuditTestSuite))
}

func (s *ResourceServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	s.resetMocks()
}

func (s *ResourceServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ResourceServiceAuditTestSuite) SetupSubTest() {
	s.resetMocks()
}

func (s *ResourceServiceAuditTestSuite) resetMocks() {
	s.store = newResourceStoreInterfaceMock(s.T())
	s.ou = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.project = newProjectOUValidatorMock(s.T())
	s.store.On("IsResourceServerDeclarative", mock.Anything).Return(false).Maybe()
}

// newService returns a resource service over the store mock that publishes on bus.
func (s *ResourceServiceAuditTestSuite) newService(bus eventbus.EventBusInterface) ResourceServiceInterface {
	service, err := newResourceService(s.ou, s.store, &fakeTransactioner{}, nil, nil, s.project, bus)
	s.Require().NoError(err)
	service.SetDependencyRegistry(resourcedependency.Initialize(service))
	return service
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

func auditedServer() providers.ResourceServer {
	return providers.ResourceServer{Name: testAuditServerName, Identifier: testAuditIdentifier, OUID: "ou-1"}
}

func auditedResource() providers.Resource {
	return providers.Resource{Name: testAuditResourceName, Handle: testAuditHandle}
}

func auditedAction() providers.Action {
	return providers.Action{Name: testAuditActionName, Handle: testAuditActionHandle}
}

// storedServer is the resource server the store holds.
func storedServer() providers.ResourceServer {
	server := auditedServer()
	server.ID = testAuditServerID
	server.Delimiter = ":"
	return server
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the target, and that it records nothing of it but its ID.
func (s *ResourceServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, target audit.Ref, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(target, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "nothing of the resource server is recorded but its ID")
	if outcome.Result == audit.ResultFailed || target.ID == "" {
		s.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name nothing that exists")
	} else {
		s.Equal(target, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditServerName, testAuditIdentifier, testAuditResourceName,
		testAuditHandle, testAuditActionName, testAuditActionHandle} {
		s.NotContains(recorded, value, "no name, identifier, handle or permission is recorded")
	}
}

func (s *ResourceServiceAuditTestSuite) TestCreateResourceServer_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the resource server is created",
			arrange: func() {
				s.ou.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil)
				s.store.On("CheckResourceServerNameExists", mock.Anything, testAuditServerName).Return(false, nil)
				s.store.On("CheckResourceServerIdentifierExists", mock.Anything, testAuditIdentifier).
					Return(false, nil)
				s.store.On("CreateResourceServer", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the name is taken",
			arrange: func() {
				s.ou.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil)
				s.store.On("CheckResourceServerNameExists", mock.Anything, testAuditServerName).Return(true, nil)
			},
			outcome: audit.OutcomeFailed("res_1004"),
		},
		{
			name: "the store fails",
			arrange: func() {
				s.ou.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil)
				s.store.On("CheckResourceServerNameExists", mock.Anything, testAuditServerName).Return(false, nil)
				s.store.On("CheckResourceServerIdentifierExists", mock.Anything, testAuditIdentifier).
					Return(false, nil)
				s.store.On("CreateResourceServer", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			created, svcErr := service.CreateResourceServer(adminContext(), auditedServer())

			target := audit.Ref{Type: audit.TargetTypeResourceServer}
			if tc.outcome == audit.OutcomeSucceeded {
				s.Require().Nil(svcErr)
				target.ID = created.ID
				s.Require().NotEmpty(target.ID)
			} else {
				s.Require().NotNil(svcErr)
			}
			s.requireOnlyEvent(published(), audit.EventResourceServerCreate, audit.ActionCreate, target, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestUpdateResourceServer_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the resource server is updated",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.ou.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil)
				s.store.On("UpdateResourceServer", mock.Anything, testAuditServerID, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the resource server does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			outcome: audit.OutcomeFailed("res_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			_, svcErr := service.UpdateResourceServer(adminContext(), testAuditServerID, auditedServer())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerUpdate, audit.ActionUpdate,
				audit.Ref{Type: audit.TargetTypeResourceServer, ID: testAuditServerID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestAProjectRefusalPublishesOneFailedEvent() {
	cases := []struct {
		name      string
		validated *tidcommon.ServiceError
		outcome   audit.Outcome
	}{
		{
			name:      "the project does not exist",
			validated: &tidcommon.ServiceError{Code: projectscope.ProjectNotFoundErrorCode},
			outcome:   audit.OutcomeFailed("res_1025"),
		},
		{
			name:      "the organization unit is outside the project",
			validated: &tidcommon.ServiceError{Code: projectscope.OUNotInProjectErrorCode},
			outcome:   audit.OutcomeFailed("res_1026"),
		},
	}
	for _, tc := range cases {
		s.Run("a create where "+tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.project.On("ValidateProjectOU", mock.Anything, "project-1", "ou-1").Return(tc.validated)
			server := auditedServer()
			server.ProjectID = "project-1"

			_, svcErr := service.CreateResourceServer(adminContext(), server)

			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventResourceServerCreate, audit.ActionCreate,
				audit.Ref{Type: audit.TargetTypeResourceServer}, tc.outcome)
		})
		s.Run("an update where "+tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			s.project.On("ValidateProjectOU", mock.Anything, "project-1", "ou-1").Return(tc.validated)
			server := auditedServer()
			server.ProjectID = "project-1"

			_, svcErr := service.UpdateResourceServer(adminContext(), testAuditServerID, server)

			s.Require().NotNil(svcErr)
			s.requireOnlyEvent(published(), audit.EventResourceServerUpdate, audit.ActionUpdate,
				audit.Ref{Type: audit.TargetTypeResourceServer, ID: testAuditServerID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestDeleteResourceServer_PublishesOneEventPerAttempt() {
	cases := []struct {
		name            string
		hasDependencies bool
		outcome         audit.Outcome
	}{
		{name: "the resource server is deleted", outcome: audit.OutcomeSucceeded},
		{name: "resources still depend on it", hasDependencies: true, outcome: audit.OutcomeFailed("res_1006")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			s.store.On("CheckResourceServerHasDependencies", mock.Anything, testAuditServerID).
				Return(tc.hasDependencies, nil)
			if tc.outcome == audit.OutcomeSucceeded {
				s.store.On("DeleteResourceServer", mock.Anything, testAuditServerID).Return(nil)
			}

			svcErr := service.DeleteResourceServer(adminContext(), testAuditServerID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerDelete, audit.ActionDelete,
				audit.Ref{Type: audit.TargetTypeResourceServer, ID: testAuditServerID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestCreateResource_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the resource is created",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.store.On("CheckResourceHandleExists", mock.Anything, testAuditServerID, testAuditHandle,
					(*string)(nil)).Return(false, nil)
				s.store.On("CreateResource", mock.Anything, mock.Anything, testAuditServerID, (*string)(nil),
					mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the resource server does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			outcome: audit.OutcomeFailed("res_1003"),
		},
		{
			name: "the handle is taken",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.store.On("CheckResourceHandleExists", mock.Anything, testAuditServerID, testAuditHandle,
					(*string)(nil)).Return(true, nil)
			},
			outcome: audit.OutcomeFailed("res_1014"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			created, svcErr := service.CreateResource(adminContext(), testAuditServerID, auditedResource())

			target := audit.Ref{Type: audit.TargetTypeResource}
			if tc.outcome == audit.OutcomeSucceeded {
				s.Require().Nil(svcErr)
				target.ID = created.ID
				s.Require().NotEmpty(target.ID)
			} else {
				s.Require().NotNil(svcErr)
			}
			s.requireOnlyEvent(published(), audit.EventResourceServerResourceCreate, audit.ActionCreate, target,
				tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestUpdateResource_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		err     error
		outcome audit.Outcome
	}{
		{name: "the resource is updated", outcome: audit.OutcomeSucceeded},
		{name: "the resource does not exist", err: errResourceNotFound, outcome: audit.OutcomeFailed("res_1008")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			stored := auditedResource()
			stored.ID = testAuditResourceID
			s.store.On("GetResource", mock.Anything, testAuditResourceID, testAuditServerID).Return(stored, tc.err)
			if tc.outcome == audit.OutcomeSucceeded {
				s.store.On("UpdateResource", mock.Anything, testAuditResourceID, testAuditServerID, mock.Anything).
					Return(nil)
			}

			_, svcErr := service.UpdateResource(adminContext(), testAuditServerID, testAuditResourceID,
				auditedResource())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerResourceUpdate, audit.ActionUpdate,
				audit.Ref{Type: audit.TargetTypeResource, ID: testAuditResourceID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestDeleteResource_PublishesOneEventPerAttempt() {
	cases := []struct {
		name            string
		hasDependencies bool
		outcome         audit.Outcome
	}{
		{name: "the resource is deleted", outcome: audit.OutcomeSucceeded},
		{name: "actions still depend on it", hasDependencies: true, outcome: audit.OutcomeFailed("res_1006")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			s.store.On("GetResource", mock.Anything, testAuditResourceID, testAuditServerID).
				Return(auditedResource(), nil)
			s.store.On("CheckResourceHasDependencies", mock.Anything, testAuditResourceID).
				Return(tc.hasDependencies, nil)
			if tc.outcome == audit.OutcomeSucceeded {
				s.store.On("DeleteResource", mock.Anything, testAuditResourceID, testAuditServerID).Return(nil)
			}

			svcErr := service.DeleteResource(adminContext(), testAuditServerID, testAuditResourceID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerResourceDelete, audit.ActionDelete,
				audit.Ref{Type: audit.TargetTypeResource, ID: testAuditResourceID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestCreateAction_PublishesOneEventPerAttempt() {
	resourceID := testAuditResourceID
	cases := []struct {
		name       string
		resourceID *string
		arrange    func()
		outcome    audit.Outcome
	}{
		{
			name: "an action of the resource server is created",
			arrange: func() {
				s.store.On("CheckActionHandleExists", mock.Anything, testAuditServerID, (*string)(nil),
					testAuditActionHandle).Return(false, nil)
				s.store.On("CreateAction", mock.Anything, mock.Anything, testAuditServerID, (*string)(nil),
					mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:       "the resource does not exist",
			resourceID: &resourceID,
			arrange: func() {
				s.store.On("GetResource", mock.Anything, testAuditResourceID, testAuditServerID).
					Return(providers.Resource{}, errResourceNotFound)
			},
			outcome: audit.OutcomeFailed("res_1008"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			tc.arrange()

			created, svcErr := service.CreateAction(adminContext(), testAuditServerID, tc.resourceID, auditedAction())

			target := audit.Ref{Type: audit.TargetTypeAction}
			if tc.outcome == audit.OutcomeSucceeded {
				s.Require().Nil(svcErr)
				target.ID = created.ID
				s.Require().NotEmpty(target.ID)
			} else {
				s.Require().NotNil(svcErr)
			}
			s.requireOnlyEvent(published(), audit.EventResourceServerActionCreate, audit.ActionCreate, target,
				tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestUpdateAction_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		err     error
		outcome audit.Outcome
	}{
		{name: "the action is updated", outcome: audit.OutcomeSucceeded},
		{name: "the action does not exist", err: errActionNotFound, outcome: audit.OutcomeFailed("res_1009")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			s.store.On("GetAction", mock.Anything, testAuditActionID, testAuditServerID, (*string)(nil)).
				Return(auditedAction(), tc.err)
			if tc.outcome == audit.OutcomeSucceeded {
				s.store.On("UpdateAction", mock.Anything, testAuditActionID, testAuditServerID, (*string)(nil),
					mock.Anything).Return(nil)
			}

			_, svcErr := service.UpdateAction(adminContext(), testAuditServerID, nil, testAuditActionID,
				auditedAction())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerActionUpdate, audit.ActionUpdate,
				audit.Ref{Type: audit.TargetTypeAction, ID: testAuditActionID}, tc.outcome)
		})
	}
}

func (s *ResourceServiceAuditTestSuite) TestDeleteAction_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		err     error
		preErr  *tidcommon.ServiceError
		postErr *tidcommon.ServiceError
		outcome audit.Outcome
	}{
		{name: "the action is deleted", outcome: audit.OutcomeSucceeded},
		{name: "the store fails", err: errors.New("store failure"), outcome: audit.OutcomeFailed("sse_5000")},
		{name: "the system flow before the deletion refuses the caller", preErr: &tidcommon.ErrorUnauthorized,
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "the system flow after the deletion fails", postErr: &tidcommon.InternalServerError,
			outcome: audit.OutcomeSucceeded},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			flows := newSystemFlowProviderMock(s.T())
			s.Require().NoError(SetSystemFlowProvider(service, flows))
			flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowActionDeletePre)).
				Return(&flowcommon.SystemFlowResult{}, tc.preErr).Once()
			s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
			s.store.On("IsActionExist", mock.Anything, testAuditActionID, testAuditServerID, (*string)(nil)).
				Return(true, nil)
			if tc.preErr == nil {
				s.store.On("DeleteAction", mock.Anything, testAuditActionID, testAuditServerID, (*string)(nil)).
					Return(tc.err)
			}
			if tc.preErr == nil && tc.err == nil {
				flows.EXPECT().ExecuteSystemFlow(mock.Anything, isHandle(systemFlowRevocationPost)).
					Return(&flowcommon.SystemFlowResult{}, tc.postErr).Once()
			}

			svcErr := service.DeleteAction(adminContext(), testAuditServerID, nil, testAuditActionID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded && tc.postErr == nil, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventResourceServerActionDelete, audit.ActionDelete,
				audit.Ref{Type: audit.TargetTypeAction, ID: testAuditActionID}, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds nothing to remove records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses. The store reports what is missing as the database store
// does: a not-found error for a resource server or a resource, and false for an action.
func (s *ResourceServiceAuditTestSuite) TestADeleteOfSomethingMissingPublishesNothing() {
	resourceID := testAuditResourceID
	cases := []struct {
		name    string
		arrange func()
		delete  func(ResourceServiceInterface) *tidcommon.ServiceError
	}{
		{
			name: "a resource server that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteResourceServer(adminContext(), testAuditServerID)
			},
		},
		{
			name: "a resource of a resource server that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteResource(adminContext(), testAuditServerID, testAuditResourceID)
			},
		},
		{
			name: "a resource that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.store.On("GetResource", mock.Anything, testAuditResourceID, testAuditServerID).
					Return(providers.Resource{}, errResourceNotFound)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteResource(adminContext(), testAuditServerID, testAuditResourceID)
			},
		},
		{
			name: "an action of a resource server that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).
					Return(providers.ResourceServer{}, errResourceServerNotFound)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteAction(adminContext(), testAuditServerID, nil, testAuditActionID)
			},
		},
		{
			name: "an action of a resource that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.store.On("GetResource", mock.Anything, testAuditResourceID, testAuditServerID).
					Return(providers.Resource{}, errResourceNotFound)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteAction(adminContext(), testAuditServerID, &resourceID, testAuditActionID)
			},
		},
		{
			name: "an action that does not exist",
			arrange: func() {
				s.store.On("GetResourceServer", mock.Anything, testAuditServerID).Return(storedServer(), nil)
				s.store.On("IsActionExist", mock.Anything, testAuditActionID, testAuditServerID, (*string)(nil)).
					Return(false, nil)
			},
			delete: func(service ResourceServiceInterface) *tidcommon.ServiceError {
				return service.DeleteAction(adminContext(), testAuditServerID, nil, testAuditActionID)
			},
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			s.Nil(tc.delete(service))
			s.Empty(published())
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *ResourceServiceAuditTestSuite) TestCreateResourceServer_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service := s.newService(failing)
	s.ou.On("GetOrganizationUnit", mock.Anything, "ou-1").Return(oupkg.OrganizationUnit{ID: "ou-1"}, nil)
	s.store.On("CheckResourceServerNameExists", mock.Anything, testAuditServerName).Return(false, nil)
	s.store.On("CheckResourceServerIdentifierExists", mock.Anything, testAuditIdentifier).Return(false, nil)
	s.store.On("CreateResourceServer", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	created, svcErr := service.CreateResourceServer(adminContext(), auditedServer())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *ResourceServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), s.ou, nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(exporter)
}
