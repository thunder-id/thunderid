// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID  = "admin-1"
	testAuditAppName  = "Audited Imported App"
	testAuditAppID    = "audited-app-1"
	testAuditPassword = "audit-import-password-0123456789"
)

// testAuditAppDocument is an application document whose ID and name no audit event may carry.
var testAuditAppDocument = fmt.Sprintf("resource_type: application\nid: %s\nname: %s\nauthFlowId: flow-1\n",
	testAuditAppID, testAuditAppName)

// publishedEvent is one audit event the import service published.
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
	require.NoError(t, audit.Subscribe(registry, "import-service-test", sink))
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

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "", "", nil, nil))
}

type ImportServiceAuditTestSuite struct {
	suite.Suite
	apps *fakeApplicationService
}

func TestImportServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ImportServiceAuditTestSuite))
}

func (s *ImportServiceAuditTestSuite) SetupTest() {
	s.apps = &fakeApplicationService{existing: map[string]*providers.Application{}}
}

func (s *ImportServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

// service returns an import service over the fake application service that publishes on bus.
func (s *ImportServiceAuditTestSuite) service(bus eventbus.EventBusInterface) ImportServiceInterface {
	svc := newTestImportService(s.apps)
	svc.(*importService).bus = bus
	return svc
}

// requireOnlyEvent checks that exactly one event was published, named name, for a request by the admin
// on the configuration as a whole, and returns it. The configuration has no ID, so no event has a
// subject, and none carries anything the request named.
func (s *ImportServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, outcome audit.Outcome) publishedEvent {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	s.Equal(name, event.name)
	s.Equal(audit.Ref{Type: audit.TargetTypeConfiguration}, event.payload.Target())
	s.Equal(audit.Ref{}, event.payload.Subject(), "the configuration as a whole is no subject")
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, carried := range []string{testAuditAppID, testAuditAppName, testAuditPassword} {
		s.NotContains(recorded, carried, "nothing the request carries is recorded")
	}
	return event
}

// An import is recorded once with its outcome and whether it was a dry run: succeeded when every item
// succeeded, failed with item_failed when one failed, and failed with the error code when the request
// was refused before any item was imported.
func (s *ImportServiceAuditTestSuite) TestImportResources_PublishesOneEventPerRequest() {
	cases := []struct {
		name      string
		content   string
		dryRun    bool
		updateErr *tidcommon.ServiceError
		existing  bool
		outcome   audit.Outcome
		svcErr    bool
	}{
		{name: "every item imported", content: testAuditAppDocument, outcome: audit.OutcomeSucceeded},
		{name: "a dry run", content: testAuditAppDocument, dryRun: true, outcome: audit.OutcomeSucceeded},
		{name: "an item failed", content: testAuditAppDocument, existing: true,
			updateErr: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "APP-1007"},
			outcome:   audit.OutcomeFailed("item_failed")},
		{name: "a dry run with an item failed", content: "resource_type: user\nid: u-1\ntype: person\n", dryRun: true,
			outcome: audit.OutcomeFailed("item_failed")},
		{name: "an empty request", outcome: audit.OutcomeFailed("imp_1001"), svcErr: true},
		{name: "content that is not YAML", content: "resource_type: [", outcome: audit.OutcomeFailed("imp_1002"),
			svcErr: true},
		{name: "a template that does not resolve", content: "name: {{.MISSING", dryRun: true,
			outcome: audit.OutcomeFailed("imp_1003"), svcErr: true},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			if tc.existing {
				s.apps.existing[testAuditAppID] = &providers.Application{ID: testAuditAppID}
			}
			s.apps.updateErr = tc.updateErr
			bus, published := newRecordingBus(s.T())

			_, svcErr := s.service(bus).ImportResources(adminContext(), &ImportRequest{
				Content: tc.content,
				DryRun:  tc.dryRun,
			})

			s.Equal(tc.svcErr, svcErr != nil)
			event := s.requireOnlyEvent(published(), audit.EventConfigurationImport, audit.ActionImport, tc.outcome)
			s.Equal(map[string]string{audit.DetailDryRun: fmt.Sprint(tc.dryRun)}, event.payload.Details())
		})
	}
}

// A dry run writes nothing, so no resource service is asked to write and the only event is the import's.
func (s *ImportServiceAuditTestSuite) TestImportResources_ADryRunWritesNothing() {
	bus, published := newRecordingBus(s.T())

	response, svcErr := s.service(bus).ImportResources(adminContext(), &ImportRequest{
		Content:   testAuditAppDocument,
		DryRun:    true,
		Deletions: []ResourceDeletion{{ResourceType: resourceTypeApplication, ID: "app-to-remove"}},
	})

	s.Require().Nil(svcErr)
	s.Equal(0, response.Summary.Failed)
	s.Empty(s.apps.sequence, "a dry run asks no service to write")
	s.Empty(s.apps.updateAttempts, "a dry run asks no service to update")
	event := s.requireOnlyEvent(published(), audit.EventConfigurationImport, audit.ActionImport,
		audit.OutcomeSucceeded)
	s.Equal(map[string]string{audit.DetailDryRun: "true"}, event.payload.Details())
}

// A credential in an imported document is never recorded.
func (s *ImportServiceAuditTestSuite) TestImportResources_RecordsNoCredential() {
	bus, published := newRecordingBus(s.T())

	_, svcErr := s.service(bus).ImportResources(adminContext(), &ImportRequest{
		Content: fmt.Sprintf("resource_type: user\nid: u-1\ntype: person\ncredentials:\n  password: %s\n",
			testAuditPassword),
	})

	s.Require().Nil(svcErr)
	s.requireOnlyEvent(published(), audit.EventConfigurationImport, audit.ActionImport,
		audit.OutcomeFailed("item_failed"))
}

// A bus that cannot take the event never fails the import it describes.
func (s *ImportServiceAuditTestSuite) TestImportResources_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()

	response, svcErr := s.service(failing).ImportResources(adminContext(), &ImportRequest{
		Content: testAuditAppDocument,
	})

	s.Require().Nil(svcErr)
	s.Equal(1, response.Summary.Imported)
	s.Len(s.apps.created, 1)
}

// A delete of a declarative resource file is recorded once, naming neither the type nor the key.
func (s *ImportServiceAuditTestSuite) TestDeleteResource_PublishesOneEventPerRequest() {
	tempHome := s.T().TempDir()
	config.ResetServerRuntime()
	s.T().Cleanup(config.ResetServerRuntime)
	s.Require().NoError(config.InitializeServerRuntime(tempHome, &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))
	resourceDir := filepath.Join(tempHome, "config", "resources", "applications")
	s.Require().NoError(os.MkdirAll(resourceDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(resourceDir, "app.yaml"), []byte(testAuditAppDocument),
		0o600))

	cases := []struct {
		name    string
		request *DeleteResourceRequest
		outcome audit.Outcome
	}{
		{"a file that exists", &DeleteResourceRequest{ResourceType: resourceTypeApplication,
			ResourceKey: testAuditAppID}, audit.OutcomeSucceeded},
		{"a file that does not exist", &DeleteResourceRequest{ResourceType: resourceTypeApplication,
			ResourceKey: testAuditAppID}, audit.OutcomeFailed("imp_1001")},
		{"a request without a key", &DeleteResourceRequest{ResourceType: resourceTypeApplication},
			audit.OutcomeFailed("imp_1001")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())

			_, svcErr := s.service(bus).DeleteResource(adminContext(), tc.request)

			s.Equal(tc.outcome.Result == audit.ResultFailed, svcErr != nil)
			event := s.requireOnlyEvent(published(), audit.EventConfigurationDelete, audit.ActionDelete,
				tc.outcome)
			s.Empty(event.payload.Details())
			s.NotContains(fmt.Sprintf("%+v", event.payload), resourceTypeApplication,
				"the resource type is not recorded")
		})
	}
}

func (s *ImportServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(http.NewServeMux(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
