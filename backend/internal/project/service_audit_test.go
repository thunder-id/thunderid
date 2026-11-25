// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

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

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditDisplayName = "Audited Storefront"
	testAuditDescription = "Audited storefront description"
)

// publishedEvent is one audit event the project service published.
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
	require.NoError(t, audit.Subscribe(registry, "project-service-test", sink))
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

// ProjectServiceAuditTestSuite checks the audit event each project change publishes.
type ProjectServiceAuditTestSuite struct {
	suite.Suite
	store    *projectStoreInterfaceMock
	entities *projectEntityReaderMock
}

func TestProjectServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectServiceAuditTestSuite))
}

func (s *ProjectServiceAuditTestSuite) SetupSubTest() {
	s.store = newProjectStoreInterfaceMock(s.T())
	s.entities = newProjectEntityReaderMock(s.T())
}

// service returns a project service over the suite's store, publishing on bus.
func (s *ProjectServiceAuditTestSuite) service(bus eventbus.EventBusInterface) *projectService {
	tx := transactionmock.NewTransactionerMock(s.T())
	tx.On("Transact", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Maybe()
	return newProjectService(s.store, tx, oumock.NewOrganizationUnitServiceInterfaceMock(s.T()), s.entities, bus)
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "", "", nil, nil))
}

// auditedRequest is a request whose display name and description no event may carry.
func auditedRequest() ProjectRequest {
	return ProjectRequest{Handle: testHandle, DisplayName: testAuditDisplayName, Description: testAuditDescription}
}

// stored is the project the store holds.
func stored() Project {
	return Project{ID: testProjectID, Handle: testHandle, DisplayName: testAuditDisplayName,
		Description: testAuditDescription, CreatedAt: testCreatedAt}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the project with projectID, with nothing of the project recorded. A succeeded attempt has the
// project as subject; a failed one has none.
func (s *ProjectServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, projectID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	project := audit.Ref{Type: audit.TargetTypeProject, ID: projectID}
	s.Equal(name, event.name)
	s.Equal(project, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed || projectID == "" {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(project, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditDisplayName, testAuditDescription, testHandle} {
		s.NotContains(recorded, value, "no attribute of the project is recorded")
	}
}

func (s *ProjectServiceAuditTestSuite) TestCreateProject_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		request func() ProjectRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the project is created",
			request: auditedRequest,
			arrange: func() {
				s.store.On("GetProjectByHandle", mock.Anything, testHandle).Return(Project{}, errProjectNotFound)
				s.store.On("CreateProject", mock.Anything, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the ID is not a UUID",
			request: func() ProjectRequest {
				request := auditedRequest()
				request.ID = "not-a-uuid"
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("prj_1005"),
		},
		{
			name: "the request has no display name",
			request: func() ProjectRequest {
				request := auditedRequest()
				request.DisplayName = " "
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("prj_1003"),
		},
		{
			name:    "another project has the handle",
			request: auditedRequest,
			arrange: func() {
				s.store.On("GetProjectByHandle", mock.Anything, testHandle).Return(stored(), nil)
			},
			outcome: audit.OutcomeFailed("prj_1012"),
		},
		{
			name:    "the store fails",
			request: auditedRequest,
			arrange: func() {
				s.store.On("GetProjectByHandle", mock.Anything, testHandle).Return(Project{}, errProjectNotFound)
				s.store.On("CreateProject", mock.Anything, mock.Anything).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			created, svcErr := s.service(bus).CreateProject(adminContext(), tc.request())

			var projectID string
			if svcErr == nil {
				projectID = created.ID
			}
			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventProjectCreate, audit.ActionCreate, projectID, tc.outcome)
		})
	}
}

func (s *ProjectServiceAuditTestSuite) TestUpdateProject_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the project is updated",
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(stored(), nil)
				s.store.On("UpdateProject", mock.Anything, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the project does not exist",
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
			},
			outcome: audit.OutcomeFailed("prj_1002"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			_, svcErr := s.service(bus).UpdateProject(adminContext(), testProjectID, auditedRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventProjectUpdate, audit.ActionUpdate, testProjectID, tc.outcome)
		})
	}
}

func (s *ProjectServiceAuditTestSuite) TestDeleteProject_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the project is deleted",
			arrange: func() {
				s.entities.On("ProjectHasEntities", mock.Anything, testProjectID).Return(false, nil)
				s.store.On("DeleteProject", mock.Anything, testProjectID).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "an application is still in the project",
			arrange: func() {
				s.entities.On("ProjectHasEntities", mock.Anything, testProjectID).Return(true, nil)
			},
			outcome: audit.OutcomeFailed("prj_1013"),
		},
		{
			name: "the store fails",
			arrange: func() {
				s.entities.On("ProjectHasEntities", mock.Anything, testProjectID).Return(false, nil)
				s.store.On("DeleteProject", mock.Anything, testProjectID).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			svcErr := s.service(bus).DeleteProject(adminContext(), testProjectID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventProjectDelete, audit.ActionDelete, testProjectID, tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no project records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The store reports the missing row as the real one does.
func (s *ProjectServiceAuditTestSuite) TestDeleteProject_PublishesNothingForAMissingProject() {
	s.Run("no project has the ID", func() {
		bus, published := newRecordingBus(s.T())
		s.entities.On("ProjectHasEntities", mock.Anything, testProjectID).Return(false, nil)
		s.store.On("DeleteProject", mock.Anything, testProjectID).Return(errProjectNotFound)

		svcErr := s.service(bus).DeleteProject(adminContext(), testProjectID)

		s.Nil(svcErr)
		s.Empty(published())
	})
}

// An import writes one event: an update when it updated, or tried to update, a project that exists, and
// a create otherwise, including an import refused before it looked the project up.
func (s *ProjectServiceAuditTestSuite) TestImportProject_PublishesOneEventPerAttempt() {
	cases := []struct {
		name      string
		id        string
		upsert    bool
		arrange   func()
		event     string
		action    audit.Action
		projectID string
		outcome   audit.Outcome
	}{
		{
			name: "a new project is created", id: testProjectID, upsert: true,
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(Project{}, errProjectNotFound)
				s.store.On("GetProjectByHandle", mock.Anything, testHandle).Return(Project{}, errProjectNotFound)
				s.store.On("CreateProject", mock.Anything, mock.Anything).Return(nil)
			},
			event: audit.EventProjectCreate, action: audit.ActionCreate, projectID: testProjectID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "an existing project is updated", id: testProjectID, upsert: true,
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(stored(), nil)
				s.store.On("UpdateProject", mock.Anything, mock.Anything).Return(nil)
			},
			event: audit.EventProjectUpdate, action: audit.ActionUpdate, projectID: testProjectID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "an existing project cannot be updated", id: testProjectID, upsert: true,
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(stored(), nil)
				s.store.On("UpdateProject", mock.Anything, mock.Anything).Return(errors.New("store failure"))
			},
			event: audit.EventProjectUpdate, action: audit.ActionUpdate, projectID: testProjectID,
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "an existing project without upsert", id: testProjectID,
			arrange: func() {
				s.store.On("GetProject", mock.Anything, testProjectID).Return(stored(), nil)
			},
			event: audit.EventProjectCreate, action: audit.ActionCreate,
			outcome: audit.OutcomeFailed("prj_1006"),
		},
		{
			name: "the ID is not a UUID", id: "not-a-uuid", upsert: true,
			arrange: func() {},
			event:   audit.EventProjectCreate, action: audit.ActionCreate,
			outcome: audit.OutcomeFailed("prj_1005"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			_, svcErr := s.service(bus).ImportProject(adminContext(), tc.id, testHandle, testAuditDisplayName,
				testAuditDescription, tc.upsert)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), tc.event, tc.action, tc.projectID, tc.outcome)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *ProjectServiceAuditTestSuite) TestCreateProject_SucceedsWhenTheBusRefusesTheEvent() {
	s.Run("the bus refuses the event", func() {
		failing := eventbusmock.NewEventBusInterfaceMock(s.T())
		failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
		s.store.On("GetProjectByHandle", mock.Anything, testHandle).Return(Project{}, errProjectNotFound)
		s.store.On("CreateProject", mock.Anything, mock.Anything).Return(nil)

		created, svcErr := s.service(failing).CreateProject(adminContext(), auditedRequest())

		s.Nil(svcErr)
		s.NotEmpty(created.ID)
	})
}

func (s *ProjectServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(exporter)
}
