// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outype

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
)

// publishedEvent is one audit event the organization unit type service published.
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

// newRecordingBus returns an event bus with an audit sink subscribed, and a function that stops the bus
// and returns the audit events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "ou-type-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

type OUTypeServiceAuditTestSuite struct {
	suite.Suite
}

func TestOUTypeServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(OUTypeServiceAuditTestSuite))
}

func (s *OUTypeServiceAuditTestSuite) TestRefusedCredentialAttributeIsAudited() {
	credentialSchema := json.RawMessage(`{"apiKey":{"type":"string","credential":true}}`)
	cases := []struct {
		name   string
		event  string
		action audit.Action
		id     string
		call   func(OUTypeServiceInterface) *tidcommon.ServiceError
	}{
		{"create", audit.EventOUTypeCreate, audit.ActionCreate, "",
			func(svc OUTypeServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.CreateOUType(context.Background(), CreateOUTypeRequest{
					Handle: "partner", DisplayName: "Partner", OUID: testOUID, Schema: credentialSchema,
				})
				return svcErr
			}},
		{"update", audit.EventOUTypeUpdate, audit.ActionUpdate, testOUTypeID,
			func(svc OUTypeServiceInterface) *tidcommon.ServiceError {
				_, svcErr := svc.UpdateOUType(context.Background(), testOUTypeID, UpdateOUTypeRequest{
					DisplayName: "Partner", OUID: testOUID, Schema: credentialSchema,
				})
				return svcErr
			}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			svc := newOUTypeService(entitytypemock.NewEntityTypeServiceInterfaceMock(s.T()), bus)

			s.Equal(&ErrorCredentialAttributeNotAllowed, tc.call(svc))

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.event, events[0].name)
			s.Equal(audit.Ref{Type: audit.TargetTypeOUType, ID: tc.id}, events[0].payload.Target())
			s.Equal(tc.action, events[0].payload.Action())
			s.Equal(audit.OutcomeFailed("out_1009"), events[0].payload.Outcome())
		})
	}
}
