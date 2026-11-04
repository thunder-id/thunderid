// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

type StatusStoreTestSuite struct {
	suite.Suite
	providerMock *providermock.DBProviderInterfaceMock
	dbClientMock *providermock.DBClientInterfaceMock
	store        *statusStore
}

func TestStatusStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StatusStoreTestSuite))
}

func (s *StatusStoreTestSuite) SetupTest() {
	s.providerMock = providermock.NewDBProviderInterfaceMock(s.T())
	s.dbClientMock = providermock.NewDBClientInterfaceMock(s.T())
	s.store = &statusStore{dbProvider: s.providerMock}
	s.providerMock.On("GetConfigDBClient").Return(s.dbClientMock, nil).Maybe()
}

// A status is read for the gateway within the deployment; a gateway never checked has none, and one
// never provisioned has no time for it.
func (s *StatusStoreTestSuite) TestGetReadsTheGatewaysRow() {
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayStatus, "gw-1", storeDeploymentID).
		Return([]map[string]interface{}{{"reachable": true, "checked_at": "2026-11-03 12:00:05",
			"provisioned_at": "2026-11-03 11:58:00"}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayStatus, "gw-2", storeDeploymentID).
		Return([]map[string]interface{}{{"reachable": int64(0), "checked_at": "2026-11-03 12:00:05",
			"provisioned_at": nil}}, nil).Once()
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayStatus, "gw-3", storeDeploymentID).
		Return([]map[string]interface{}{}, nil).Once()

	status, err := s.store.Get(scoped(), "gw-1")
	s.Require().NoError(err)
	s.True(status.Reachable)
	s.Equal(time.Date(2026, 11, 3, 12, 0, 5, 0, time.UTC), status.CheckedAt)
	s.Require().NotNil(status.ProvisionedAt)
	s.Equal(time.Date(2026, 11, 3, 11, 58, 0, 0, time.UTC), *status.ProvisionedAt)

	status, err = s.store.Get(scoped(), "gw-2")
	s.Require().NoError(err)
	s.False(status.Reachable)
	s.Nil(status.ProvisionedAt)

	status, err = s.store.Get(scoped(), "gw-3")
	s.Require().NoError(err)
	s.Nil(status)
}

// A check is recorded, and a removed gateway forgotten, within the deployment.
func (s *StatusStoreTestSuite) TestRecordAndDeleteAreDeploymentScoped() {
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryRecordGatewayStatus, "gw-1", true, storeDeploymentID).
		Return(int64(1), nil).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteGatewayStatus, "gw-1", storeDeploymentID).
		Return(int64(1), nil).Once()

	s.Require().NoError(s.store.Record(scoped(), "gw-1", true))
	s.Require().NoError(s.store.Delete(scoped(), "gw-1"))
}

// A database that cannot answer fails each call.
func (s *StatusStoreTestSuite) TestDatabaseFailuresAreReported() {
	down := errors.New("database is down")
	s.dbClientMock.On("QueryContext", mock.Anything, queryGetGatewayStatus, mock.Anything, mock.Anything).
		Return(nil, down).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryRecordGatewayStatus, mock.Anything, mock.Anything,
		mock.Anything).Return(int64(0), down).Once()
	s.dbClientMock.On("ExecuteContext", mock.Anything, queryDeleteGatewayStatus, mock.Anything, mock.Anything).
		Return(int64(0), down).Once()

	_, err := s.store.Get(scoped(), "gw-1")
	s.Error(err)
	s.Error(s.store.Record(scoped(), "gw-1", false))
	s.Error(s.store.Delete(scoped(), "gw-1"))
}

// Without a database client, each call fails.
func (s *StatusStoreTestSuite) TestAMissingClientIsReported() {
	providerMock := providermock.NewDBProviderInterfaceMock(s.T())
	providerMock.On("GetConfigDBClient").Return(nil, errors.New("no client"))
	store := &statusStore{dbProvider: providerMock}

	_, err := store.Get(scoped(), "gw-1")
	s.Error(err)
	s.Error(store.Record(scoped(), "gw-1", true))
	s.Error(store.Delete(scoped(), "gw-1"))
}

// readinessWith builds readiness over the given gateways that a test drives through its status store
// and prober.
func readinessWith(gateways *fakeStore) (*readiness, *fakeStatusStore, *fakeProber) {
	statuses := newFakeStatusStore(time.Now)
	prober := &fakeProber{err: errors.New("connection refused")}
	return newReadiness(gateways, statuses, prober), statuses, prober
}

// A plane that is not a control plane has no readiness, and does not serve the status at all.
func TestTheDefaultStatusRouteIsServedOnAControlPlaneAlone(t *testing.T) {
	mux := http.NewServeMux()
	registerRoutes(mux, newHandler(&stubService{}))

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/gateways/default/status", nil))

	if recorder.Code == http.StatusOK {
		t.Fatalf("a plane without readiness served the status: %s", recorder.Body.String())
	}
}

// The default gateway's status is served to any caller, pending while there is none.
func TestTheDefaultStatusRoute(t *testing.T) {
	gateways := &fakeStore{gateways: []Gateway{{ID: "gw-1", IsDefault: true}}}
	ready, _, prober := readinessWith(gateways)
	prober.set(nil)
	mux := http.NewServeMux()
	h := newHandler(&stubService{})
	h.readiness = ready
	registerRoutes(mux, h)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/gateways/default/status", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var status GatewayStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.State != StateReady || status.ProvisionedAt == nil {
		t.Fatalf("unexpected status %+v", status)
	}

	gateways.gateways[0].IsDefault = false
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/gateways/default/status", nil))
	if body := recorder.Body.String(); recorder.Code != http.StatusOK || !json.Valid([]byte(body)) ||
		!strings.Contains(body, `"state":"pending"`) {
		t.Fatalf("unexpected answer %d: %s", recorder.Code, body)
	}

	gateways.err = errors.New("database is down")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/gateways/default/status", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("a failed read answered %d", recorder.Code)
	}
}

// A request refused until the data plane is ready is a conflict, with when to ask again.
func TestADataPlaneRefusalIsAConflictWithRetryAfter(t *testing.T) {
	for _, refusal := range []tidcommon.ServiceError{
		declarativeresource.ErrorDataPlaneNotReady, declarativeresource.ErrorNoDataPlane,
	} {
		recorder := httptest.NewRecorder()
		handleError(context.Background(), recorder, &refusal)

		if recorder.Code != http.StatusConflict {
			t.Fatalf("%s answered %d", refusal.Code, recorder.Code)
		}
		if got := recorder.Header().Get("Retry-After"); got != "15" {
			t.Fatalf("%s gave Retry-After %q", refusal.Code, got)
		}
		if !strings.Contains(recorder.Body.String(), refusal.Code) {
			t.Fatalf("%s is not in the answer: %s", refusal.Code, recorder.Body.String())
		}
	}
}
