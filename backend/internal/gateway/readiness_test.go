// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeStatusStore keeps statuses as the GATEWAY_STATUS table does: the first check a gateway answers
// sets when it was provisioned, and no later check clears it.
type fakeStatusStore struct {
	mu      sync.Mutex
	rows    map[string]*checkedStatus
	now     func() time.Time
	err     error
	deleted []string
}

func newFakeStatusStore(now func() time.Time) *fakeStatusStore {
	return &fakeStatusStore{rows: map[string]*checkedStatus{}, now: now}
}

func (f *fakeStatusStore) Get(_ context.Context, gatewayID string) (*checkedStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	row, ok := f.rows[gatewayID]
	if !ok {
		return nil, nil
	}
	copied := *row
	return &copied, nil
}

func (f *fakeStatusStore) Record(_ context.Context, gatewayID string, reachable bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	now := f.now()
	row, ok := f.rows[gatewayID]
	if !ok {
		row = &checkedStatus{}
		f.rows[gatewayID] = row
	}
	row.Reachable, row.CheckedAt = reachable, now
	if reachable && row.ProvisionedAt == nil {
		row.ProvisionedAt = &now
	}
	return nil
}

func (f *fakeStatusStore) Delete(_ context.Context, gatewayID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, gatewayID)
	delete(f.rows, gatewayID)
	return nil
}

// fakeProber answers health checks with err, counting them. With hold set, a check waits for it to
// be closed, so a test can have several requests need the same check at once.
type fakeProber struct {
	mu    sync.Mutex
	err   error
	calls int
	hold  chan struct{}
}

func (p *fakeProber) Health(context.Context, *Gateway) error {
	p.mu.Lock()
	p.calls++
	hold, err := p.hold, p.err
	p.mu.Unlock()
	if hold != nil {
		<-hold
	}
	return err
}

func (p *fakeProber) set(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *fakeProber) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type ReadinessTestSuite struct {
	suite.Suite
	at       time.Time
	statuses *fakeStatusStore
	prober   *fakeProber
	gateways *fakeStore
	ready    *readiness
}

func TestReadinessTestSuite(t *testing.T) {
	suite.Run(t, new(ReadinessTestSuite))
}

func (s *ReadinessTestSuite) SetupTest() {
	s.at = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	now := func() time.Time { return s.at }
	s.statuses = newFakeStatusStore(now)
	s.prober = &fakeProber{err: errors.New("connection refused")}
	s.gateways = &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev", IsDefault: true}}}
	s.ready = newReadiness(s.gateways, s.statuses, s.prober)
	s.ready.now = now
}

func (s *ReadinessTestSuite) gateway() *Gateway { return &s.gateways.gateways[0] }

// A gateway never checked is checked before its state is given; until it answers, it is still being
// provisioned.
func (s *ReadinessTestSuite) TestAGatewayNeverAnsweringIsProvisioning() {
	status, err := s.ready.Status(context.Background(), s.gateway())

	s.Require().NoError(err)
	s.Equal(StateProvisioning, status.State)
	s.Nil(status.ProvisionedAt)
	s.Require().NotNil(status.CheckedAt)
	s.Equal(1, s.prober.count())
}

// A gateway answering its check is ready from then on, and records when it was provisioned; one
// failing a later check is unreachable, and keeps that time.
func (s *ReadinessTestSuite) TestAGatewayGoesFromProvisioningToReadyToUnreachable() {
	_, err := s.ready.Status(context.Background(), s.gateway())
	s.Require().NoError(err)

	s.prober.set(nil)
	s.at = s.at.Add(notReadyCheckInterval)
	status, err := s.ready.Status(context.Background(), s.gateway())
	s.Require().NoError(err)
	s.Equal(StateReady, status.State)
	s.Require().NotNil(status.ProvisionedAt)
	provisioned := *status.ProvisionedAt

	s.prober.set(errors.New("timeout"))
	s.Require().NoError(s.statuses.Record(context.Background(), "gw-1", false))
	status, err = s.ready.Status(context.Background(), s.gateway())
	s.Require().NoError(err)
	s.Equal(StateUnreachable, status.State)
	s.Equal(provisioned, *status.ProvisionedAt)
}

// A state checked within its interval is given as it is, without checking again.
func (s *ReadinessTestSuite) TestAFreshStateIsNotCheckedAgain() {
	_, err := s.ready.Status(context.Background(), s.gateway())
	s.Require().NoError(err)

	s.at = s.at.Add(notReadyCheckInterval - time.Second)
	status, err := s.ready.Status(context.Background(), s.gateway())

	s.Require().NoError(err)
	s.Equal(StateProvisioning, status.State)
	s.Equal(1, s.prober.count())
}

// A stale ready state is given as it is and checked again in the background, so a request never
// waits on a check of a gateway that was already provisioned.
func (s *ReadinessTestSuite) TestAStaleReadyStateIsRefreshedInTheBackground() {
	s.prober.set(nil)
	_, err := s.ready.Status(context.Background(), s.gateway())
	s.Require().NoError(err)
	s.prober.set(errors.New("timeout"))
	hold := make(chan struct{})
	s.prober.mu.Lock()
	s.prober.hold = hold
	s.prober.mu.Unlock()

	s.at = s.at.Add(readyCheckInterval)
	status, err := s.ready.Status(context.Background(), s.gateway())

	s.Require().NoError(err)
	s.Equal(StateReady, status.State, "a stale ready state waited on its check")
	close(hold)
	s.Eventually(func() bool {
		stored, _ := s.statuses.Get(context.Background(), "gw-1")
		return stateOf(stored) == StateUnreachable
	}, time.Second, time.Millisecond)
}

// Several requests needing the same check at once share it.
func (s *ReadinessTestSuite) TestConcurrentRequestsShareOneCheck() {
	hold := make(chan struct{})
	s.prober.hold = hold
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ready.Status(context.Background(), s.gateway())
		}()
	}
	s.Eventually(func() bool { return s.prober.count() == 1 }, time.Second, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	close(hold)
	wg.Wait()

	s.Equal(1, s.prober.count())
}

// Peek never waits on a check: a gateway never checked reads as provisioning and is checked in the
// background.
func (s *ReadinessTestSuite) TestPeekChecksInTheBackground() {
	s.prober.set(nil)

	status, err := s.ready.Peek(context.Background(), s.gateway())

	s.Require().NoError(err)
	s.Equal(StateProvisioning, status.State)
	s.Nil(status.CheckedAt)
	s.Eventually(func() bool {
		stored, _ := s.statuses.Get(context.Background(), "gw-1")
		return stateOf(stored) == StateReady
	}, time.Second, time.Millisecond)
}

// Without a default gateway the data plane is pending, and a create that captures values is refused
// with a code of its own; while it is provisioning, with another; once provisioned, never, even while
// it cannot be reached.
func (s *ReadinessTestSuite) TestCreatesWaitForTheDefaultGateway() {
	s.gateways.gateways[0].IsDefault = false
	status, svcErr := s.ready.DefaultStatus(context.Background())
	s.Require().Nil(svcErr)
	s.Equal(&GatewayStatus{State: StatePending}, status)
	s.Equal(&declarativeresource.ErrorNoDataPlane, s.ready.CheckDataPlaneReady(context.Background()))

	s.gateways.gateways[0].IsDefault = true
	s.Equal(&declarativeresource.ErrorDataPlaneNotReady, s.ready.CheckDataPlaneReady(context.Background()))

	s.prober.set(nil)
	s.at = s.at.Add(notReadyCheckInterval)
	s.Nil(s.ready.CheckDataPlaneReady(context.Background()))

	s.Require().NoError(s.statuses.Record(context.Background(), "gw-1", false))
	s.Nil(s.ready.CheckDataPlaneReady(context.Background()), "an unreachable data plane refused a create")
}

// A deploy, or a call to a gateway's store, is refused only while the gateway has never answered.
func (s *ReadinessTestSuite) TestRequireProvisioned() {
	s.Equal(&declarativeresource.ErrorDataPlaneNotReady, s.ready.RequireProvisioned(context.Background(), s.gateway()))

	s.prober.set(nil)
	s.at = s.at.Add(notReadyCheckInterval)
	s.Nil(s.ready.RequireProvisioned(context.Background(), s.gateway()))
}

// A status that cannot be read fails the request rather than letting it through or refusing it as
// not ready.
func (s *ReadinessTestSuite) TestAStatusThatCannotBeReadIsAServerError() {
	s.statuses.err = errors.New("database is down")

	s.Equal(&tidcommon.InternalServerError, s.ready.CheckDataPlaneReady(context.Background()))
	s.Equal(&tidcommon.InternalServerError, s.ready.RequireProvisioned(context.Background(), s.gateway()))
	_, err := s.ready.Peek(context.Background(), s.gateway())
	s.Error(err)

	s.statuses.err = nil
	s.gateways.err = errors.New("database is down")
	_, svcErr := s.ready.DefaultStatus(context.Background())
	s.Equal(&tidcommon.InternalServerError, svcErr)
}

// A check that cannot be recorded leaves the state as it was.
func (s *ReadinessTestSuite) TestACheckThatCannotBeRecordedChangesNothing() {
	s.prober.set(nil)
	s.statuses.err = errors.New("database is down")

	s.ready.check(context.Background(), s.gateway())

	s.statuses.err = nil
	stored, err := s.statuses.Get(context.Background(), "gw-1")
	s.Require().NoError(err)
	s.Nil(stored)
}

// Forgetting a gateway drops its status.
func (s *ReadinessTestSuite) TestForgetDropsTheStatus() {
	s.ready.Forget(context.Background(), "gw-1")

	s.Equal([]string{"gw-1"}, s.statuses.deleted)
}

// The value capture refuses through the default gateway's readiness, except for an internal caller,
// and a capture not bound refuses nothing.
func (s *ReadinessTestSuite) TestTheValueCaptureRefusesThroughReadiness() {
	s.Nil((*ValueCapture)(nil).CheckDataPlaneReady(context.Background()))
	capture := NewValueCapture()
	s.Nil(capture.CheckDataPlaneReady(context.Background()))

	capture.bind(&fakeExporter{}, s.gateways, nil, s.ready)
	s.Equal(&declarativeresource.ErrorDataPlaneNotReady, capture.CheckDataPlaneReady(context.Background()))
	s.Nil(capture.CheckDataPlaneReady(security.WithRuntimeContext(context.Background())),
		"bootstrap was refused while the data plane was not ready")
}

// The health check calls the gateway's readiness endpoint, and anything but 200 is not ready.
func (s *ReadinessTestSuite) TestTheClientCallsTheReadinessEndpoint() {
	var path string
	ready := gatewayFor(s.T(), func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	s.NoError(newGatewayClient().Health(context.Background(), ready))
	s.Equal("/health/readiness", path)

	starting := gatewayFor(s.T(), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	s.Error(newGatewayClient().Health(context.Background(), starting))

	closed := gatewayFor(s.T(), func(http.ResponseWriter, *http.Request) {})
	closed.BaseURL = "https://127.0.0.1:1"
	s.Error(newGatewayClient().Health(context.Background(), closed))
}

// A gateway read carries its status; one registered or changed is checked at once, and one removed
// has its status forgotten.
func TestTheGatewayServiceKeepsReadinessUpToDate(t *testing.T) {
	store := &fakeStore{}
	svc := newTestService(t, store)
	ready, statuses, prober := readinessWith(store)
	prober.set(nil)
	svc.(*service).readiness = ready

	registered, svcErr := svc.Register(context.Background(), validRequest("dev"))
	if svcErr != nil {
		t.Fatalf("register: %v", svcErr)
	}
	waitFor(t, func() bool { return prober.count() == 1 }, "a registered gateway was not checked")
	waitFor(t, func() bool {
		stored, _ := statuses.Get(context.Background(), registered.ID)
		return stateOf(stored) == StateReady
	}, "the check was not recorded")

	gw, svcErr := svc.Get(context.Background(), registered.ID)
	if svcErr != nil || gw.Status == nil || gw.Status.State != StateReady {
		t.Fatalf("a read did not carry the status: %+v %v", gw, svcErr)
	}
	listed, svcErr := svc.List(context.Background())
	if svcErr != nil || len(listed) != 1 || listed[0].Status == nil || listed[0].Status.State != StateReady {
		t.Fatalf("a listing did not carry the status: %+v %v", listed, svcErr)
	}

	if _, svcErr := svc.Update(context.Background(), registered.ID, UpdateRequest{
		BaseURL: strptr("https://moved.example.test"),
	}); svcErr != nil {
		t.Fatalf("update: %v", svcErr)
	}
	waitFor(t, func() bool { return prober.count() == 2 }, "an updated gateway was not checked")

	if svcErr := svc.Delete(context.Background(), registered.ID); svcErr != nil {
		t.Fatalf("delete: %v", svcErr)
	}
	if len(statuses.deleted) != 1 || statuses.deleted[0] != registered.ID {
		t.Fatalf("the status was not forgotten: %v", statuses.deleted)
	}

	// A status that cannot be read is left out of the read rather than failing it.
	store.gateways = []Gateway{{ID: "gw-9", Name: "x"}}
	statuses.err = errors.New("database is down")
	gw, svcErr = svc.Get(context.Background(), "gw-9")
	if svcErr != nil || gw.Status != nil {
		t.Fatalf("an unreadable status failed the read: %+v %v", gw, svcErr)
	}
}

// A deploy, dry runs included, and a revert to a gateway never provisioned are refused before the
// gateway is called; once it answers its check, the deploy goes ahead.
func TestDeploysWaitForTheGatewayToBeProvisioned(t *testing.T) {
	f := newVersionFixture(t)
	f.capture(t, "ou-a")
	versions := f.svc.(*versionService)
	ready, statuses, _ := readinessWith(versions.gateways.(*fakeStore))
	versions.readiness = ready

	for _, dryRun := range []bool{true, false} {
		_, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: dryRun})
		if svcErr == nil || svcErr.Code != declarativeresource.ErrorDataPlaneNotReady.Code {
			t.Fatalf("a deploy (dry run %v) to a gateway being provisioned answered %v", dryRun, svcErr)
		}
	}
	if _, svcErr := f.svc.Revert(context.Background(), "gw-1", RevertRequest{}); svcErr == nil ||
		svcErr.Code != declarativeresource.ErrorDataPlaneNotReady.Code {
		t.Fatalf("a revert to a gateway being provisioned answered %v", svcErr)
	}
	if len(f.client.sent) != 0 {
		t.Fatal("a gateway being provisioned was called")
	}

	if err := statuses.Record(context.Background(), "gw-1", true); err != nil {
		t.Fatal(err)
	}
	if _, svcErr := f.svc.Apply(context.Background(), "gw-1", ApplyRequest{}); svcErr != nil {
		t.Fatalf("a deploy to a provisioned gateway was refused: %v", svcErr)
	}
}

// A call to the store of a gateway never provisioned is refused before the gateway is called.
func TestStoreCallsWaitForTheGatewayToBeProvisioned(t *testing.T) {
	gateways := &fakeStore{gateways: []Gateway{{ID: "gw-1", Name: "dev"}}}
	stores := newStoreService(gateways, newGatewayClient()).(*storeService)
	ready, _, _ := readinessWith(gateways)
	stores.readiness = ready

	_, svcErr := stores.ListVariables(context.Background(), "gw-1", StoreListQuery{})

	if svcErr == nil || svcErr.Code != declarativeresource.ErrorDataPlaneNotReady.Code {
		t.Fatalf("a store call to a gateway being provisioned answered %v", svcErr)
	}
}

func waitFor(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(time.Millisecond)
	}
}
