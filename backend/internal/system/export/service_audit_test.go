// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package export

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

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditAppName      = "Audited Exported App"
	testAuditMissingAppID = "audited-missing-app"
	testAuditSecret       = "audit-export-secret-0123456789"
)

// publishedEvent is one audit event the export service published.
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
	require.NoError(t, audit.Subscribe(registry, "export-service-test", sink))
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

type ExportServiceAuditTestSuite struct {
	suite.Suite
	apps *applicationmock.ApplicationServiceInterfaceMock
}

func TestExportServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ExportServiceAuditTestSuite))
}

func (s *ExportServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{
		Crypto: config.CryptoConfig{Encryption: engineconfig.EncryptionConfig{
			Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
		}},
	}))
	s.apps = applicationmock.NewApplicationServiceInterfaceMock(s.T())
}

func (s *ExportServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *ExportServiceAuditTestSuite) SetupSubTest() {
	s.apps = applicationmock.NewApplicationServiceInterfaceMock(s.T())
}

// service returns an export service over the mocked application service that publishes on bus.
func (s *ExportServiceAuditTestSuite) service(bus eventbus.EventBusInterface) ExportServiceInterface {
	return newExportService([]declarativeresource.ResourceExporter{
		application.NewApplicationExporterForTest(s.apps, sharingWithNoPolicies(s.T())),
	}, newParameterizer(templatingRules{}, TemplatePlaceholders), bus)
}

// holds makes the application service hold an application with an OAuth client under appID.
func (s *ExportServiceAuditTestSuite) holds(appID string) {
	s.apps.EXPECT().GetApplication(mock.Anything, appID).Return(&providers.Application{
		ID: appID, Name: testAuditAppName,
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID: "audited-client", ClientSecret: testAuditSecret,
				RedirectURIs: []string{"https://audit.example.com/callback"},
			},
		}},
	}, nil).Once()
}

// misses makes the application service hold no application under appID.
func (s *ExportServiceAuditTestSuite) misses(appID string) {
	s.apps.EXPECT().GetApplication(mock.Anything, appID).Return(nil,
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "APP-1001"}).Once()
}

// An export is recorded once: succeeded when every resource was exported, failed with item_failed
// when one could not be, and failed with the error code when nothing was. Nothing the request named or
// the export wrote is recorded.
func (s *ExportServiceAuditTestSuite) TestExportResources_PublishesOneEventPerRequest() {
	cases := []struct {
		name    string
		held    []string
		missing []string
		outcome audit.Outcome
	}{
		{name: "every resource exported", held: []string{"app-a", "app-b"}, outcome: audit.OutcomeSucceeded},
		{name: "a resource not exported", held: []string{"app-a"}, missing: []string{testAuditMissingAppID},
			outcome: audit.OutcomeFailed("item_failed")},
		{name: "no resource exported", missing: []string{testAuditMissingAppID},
			outcome: audit.OutcomeFailed("exp_1002")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			for _, id := range tc.held {
				s.holds(id)
			}
			for _, id := range tc.missing {
				s.misses(id)
			}
			bus, published := newRecordingBus(s.T())

			response, _ := s.service(bus).ExportResources(adminContext(), &ExportRequest{
				Applications: append(append([]string{}, tc.held...), tc.missing...),
			})

			events := published()
			s.Require().Len(events, 1)
			event := events[0]
			s.Equal(audit.EventConfigurationExport, event.name)
			s.Equal(audit.Ref{Type: audit.TargetTypeConfiguration}, event.payload.Target())
			s.Equal(audit.Ref{}, event.payload.Subject())
			s.Equal(audit.ActionExport, event.payload.Action())
			s.Equal(tc.outcome, event.payload.Outcome())
			s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
			s.Empty(event.payload.Details())
			recorded := fmt.Sprintf("%+v", event.payload)
			for _, carried := range append([]string{testAuditAppName, testAuditMissingAppID, "audited-client",
				"audit.example.com", testAuditSecret}, tc.held...) {
				s.NotContains(recorded, carried, "nothing the export named or wrote is recorded")
			}
			if response != nil {
				s.NotEmpty(response.Files)
			}
		})
	}
}

// A request the service refuses outright is recorded as failed with its code.
func (s *ExportServiceAuditTestSuite) TestExportResources_PublishesARefusedRequest() {
	bus, published := newRecordingBus(s.T())

	_, svcErr := s.service(bus).ExportResources(adminContext(), nil)

	s.Require().NotNil(svcErr)
	events := published()
	s.Require().Len(events, 1)
	s.Equal(audit.OutcomeFailed("exp_1001"), events[0].payload.Outcome())
}

// A read on the deployment's behalf is not an export request, so it publishes nothing.
func (s *ExportServiceAuditTestSuite) TestReadResources_PublishesNothing() {
	s.holds("app-a")
	bus := eventbusmock.NewEventBusInterfaceMock(s.T())

	response, svcErr := s.service(bus).ReadResources(adminContext(), &ExportRequest{Applications: []string{"app-a"}})

	s.Require().Nil(svcErr)
	s.Len(response.Files, 1)
}

// A bus that cannot take the event never fails the export it describes.
func (s *ExportServiceAuditTestSuite) TestExportResources_SucceedsWhenTheBusRefusesTheEvent() {
	s.holds("app-a")
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()

	response, svcErr := s.service(failing).ExportResources(adminContext(),
		&ExportRequest{Applications: []string{"app-a"}})

	s.Require().Nil(svcErr)
	s.Len(response.Files, 1)
}

func (s *ExportServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(http.NewServeMux(), nil, TemplatePlaceholders, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
}
