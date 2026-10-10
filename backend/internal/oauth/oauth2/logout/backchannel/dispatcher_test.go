// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package backchannel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/session"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/system/config"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/log/rollingfile"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/httpmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/mocks/observabilityprovidermock"
)

type DispatcherTestSuite struct {
	suite.Suite
	tokens  *tokenservicemock.TokenBuilderInterfaceMock
	clients *actorprovidermock.ActorProviderMock
	events  *observabilityprovidermock.ObservabilityProviderMock
	d       *dispatcher

	mu        sync.Mutex
	published []*providers.Event
}

func TestDispatcherTestSuite(t *testing.T) {
	suite.Run(t, new(DispatcherTestSuite))
}

func (s *DispatcherTestSuite) SetupSuite() {
	// The HTTP client reads the TLS settings from the server runtime.
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
}

func (s *DispatcherTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (s *DispatcherTestSuite) SetupTest() {
	s.tokens = tokenservicemock.NewTokenBuilderInterfaceMock(s.T())
	s.clients = actorprovidermock.NewActorProviderMock(s.T())
	s.events = observabilityprovidermock.NewObservabilityProviderMock(s.T())
	s.d = nil
	s.published = nil
	s.events.On("IsEnabled").Return(true).Maybe()
	s.events.On("PublishEvent", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.published = append(s.published, args.Get(1).(*providers.Event))
	}).Return().Maybe()
}

// snapshot returns the events published so far.
func (s *DispatcherTestSuite) snapshot() []*providers.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*providers.Event(nil), s.published...)
}

// SetupSubTest gives each table case its own mocks and dispatcher.
func (s *DispatcherTestSuite) SetupSubTest() {
	s.SetupTest()
}

func testConfig() engineconfig.BackchannelLogoutConfig {
	reject := false
	return engineconfig.BackchannelLogoutConfig{
		RequestTimeout:         2,
		MaxAttempts:            3,
		RetryDelay:             2,
		MaxInFlight:            16,
		QueueSize:              16,
		RejectPrivateAddresses: &reject,
	}
}

// build creates a stopped dispatcher whose token builder always succeeds.
func (s *DispatcherTestSuite) build(cfg engineconfig.BackchannelLogoutConfig) {
	s.tokens.On("BuildLogoutToken", mock.Anything, mock.Anything).
		Return(&oauth2model.TokenDTO{Token: "logout-token"}, nil).Maybe()
	s.buildWithTokens(cfg)
}

// buildWithTokens creates a stopped dispatcher with the token builder expectations already set. The
// retry delay and shutdown budget are shortened so the tests run in milliseconds.
func (s *DispatcherTestSuite) buildWithTokens(cfg engineconfig.BackchannelLogoutConfig) {
	httpClient := syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
		Timeout:          time.Duration(cfg.RequestTimeout) * time.Second,
		DisableRedirects: true,
		GuardSSRF:        cfg.RejectsPrivateAddresses(),
	})
	s.d = newDispatcher(cfg, s.tokens, s.clients, httpClient, s.events)
	s.d.retryDelay = 10 * time.Millisecond
	s.d.shutdownBudget = time.Second
}

func (s *DispatcherTestSuite) withClient(appID, uri string) {
	s.clients.On("GetOAuthClientByID", mock.Anything, appID).
		Return(&providers.OAuthClient{ID: appID, ClientID: "client-" + appID, BackchannelLogoutURI: uri}, nil).
		Maybe()
}

func (s *DispatcherTestSuite) start() {
	s.d.Start(context.Background())
	s.T().Cleanup(s.d.Stop)
}

// waitFor blocks until n events were published and returns them.
func (s *DispatcherTestSuite) waitFor(n int) []*providers.Event {
	s.T().Helper()
	s.Require().Eventually(func() bool { return len(s.snapshot()) >= n }, 5*time.Second, 5*time.Millisecond,
		"expected %d events", n)
	return s.snapshot()
}

func terminated(appIDs ...string) session.TerminatedSession {
	ended := session.TerminatedSession{SessionID: "session-1", SubjectID: "user-1", Reason: "sign_out"}
	for _, id := range appIDs {
		ended.Participants = append(ended.Participants, session.Participant{SessionID: "session-1", AppID: id})
	}
	return ended
}

func (s *DispatcherTestSuite) statusServer(statuses ...int) (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := int(calls.Add(1))
		status := statuses[len(statuses)-1]
		if n <= len(statuses) {
			status = statuses[n-1]
		}
		if status == http.StatusFound {
			w.Header().Set("Location", "/elsewhere")
		}
		w.WriteHeader(status)
	}))
	s.T().Cleanup(srv.Close)
	return srv, &calls
}

// hangingServer never answers until the test ends.
func (s *DispatcherTestSuite) hangingServer() (*httptest.Server, *atomic.Int32) {
	var calls atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	// Cleanups run last first: release the handlers, then close the server.
	s.T().Cleanup(srv.Close)
	s.T().Cleanup(func() { close(release) })
	return srv, &calls
}

// captureLogs sends the shared logger's output to a file while fn runs and returns what it wrote.
func (s *DispatcherTestSuite) captureLogs(level string, fn func()) string {
	path := filepath.Join(s.T().TempDir(), "dispatcher.log")
	logger := log.GetLogger()
	s.Require().NoError(logger.Configure(log.OutputOptions{FileEnabled: true, File: rollingfile.Config{Path: path}}))
	s.Require().NoError(logger.SetLevel(level))
	defer func() {
		s.Require().NoError(logger.SetLevel("INFO"))
		s.Require().NoError(logger.Configure(log.OutputOptions{ConsoleEnabled: true}))
	}()
	fn()
	// #nosec G304 -- path is in the test's temp directory
	out, err := os.ReadFile(path)
	s.Require().NoError(err)
	return string(out)
}

// endlessBody is a response body that never ends; it counts what was read and whether it was closed.
type endlessBody struct {
	read   atomic.Int64
	closed atomic.Bool
}

func (b *endlessBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	b.read.Add(int64(len(p)))
	return len(p), nil
}

func (b *endlessBody) Close() error {
	b.closed.Store(true)
	return nil
}

func (s *DispatcherTestSuite) TestDeliversLogoutToken() {
	var got *http.Request
	var form string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_ = r.ParseForm()
		form = r.PostForm.Get("logout_token")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s.build(testConfig())
	s.withClient("app-1", srv.URL+"/bcl")
	s.start()

	s.d.OnEvent(syscontext.WithTraceID(context.Background(), "trace-1"), terminated("app-1"))
	events := s.waitFor(1)

	require.NotNil(s.T(), got)
	assert.Equal(s.T(), http.MethodPost, got.Method)
	assert.Equal(s.T(), "application/x-www-form-urlencoded", got.Header.Get("Content-Type"))
	assert.Equal(s.T(), "logout-token", form)

	evt := events[0]
	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), evt.Type)
	assert.Equal(s.T(), providers.StatusSuccess, evt.Status)
	assert.Equal(s.T(), "trace-1", evt.TraceID)
	assert.Equal(s.T(), "client-app-1", evt.Data[event.DataKey.ClientID])
	assert.Equal(s.T(), "app-1", evt.Data[event.DataKey.EntityID])
	assert.Equal(s.T(), "session-1", evt.Data[event.DataKey.SessionID])
	assert.Equal(s.T(), 1, evt.Data[event.DataKey.AttemptNumber])
	assert.Equal(s.T(), http.StatusOK, evt.Data[event.DataKey.HTTPStatus])
	assert.NotContains(s.T(), evt.Data, event.DataKey.Error)

	client := &providers.OAuthClient{ID: "app-1", ClientID: "client-app-1", BackchannelLogoutURI: srv.URL + "/bcl"}
	s.tokens.AssertCalled(s.T(), "BuildLogoutToken", mock.Anything, &tokenservice.LogoutTokenBuildContext{
		OAuthApp:  client,
		SubjectID: "user-1",
		SessionID: "session-1",
	})
}

func (s *DispatcherTestSuite) TestClassification() {
	tests := []struct {
		name        string
		statuses    []int
		wantType    providers.EventType
		wantReason  failureReason
		wantAttempt int
		wantCalls   int32
	}{
		{"204 is delivered", []int{http.StatusNoContent}, event.EventTypeBackchannelLogoutDelivered, "", 1, 1},
		{"503 then 200 retries", []int{http.StatusServiceUnavailable, http.StatusOK},
			event.EventTypeBackchannelLogoutDelivered, "", 2, 2},
		{"429 without Retry-After retries", []int{http.StatusTooManyRequests, http.StatusOK},
			event.EventTypeBackchannelLogoutDelivered, "", 2, 2},
		{"5xx exhausts attempts", []int{http.StatusInternalServerError},
			event.EventTypeBackchannelLogoutFailed, reasonServerError, 3, 3},
		{"400 is not retried", []int{http.StatusBadRequest},
			event.EventTypeBackchannelLogoutFailed, reasonRejected, 1, 1},
		{"302 is not followed", []int{http.StatusFound}, event.EventTypeBackchannelLogoutFailed, reasonRejected, 1, 1},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			srv, calls := s.statusServer(tc.statuses...)
			s.build(testConfig())
			s.withClient("app-1", srv.URL)
			s.start()

			s.d.OnEvent(context.Background(), terminated("app-1"))
			evt := s.waitFor(1)[0]

			assert.Equal(s.T(), string(tc.wantType), evt.Type)
			assert.Equal(s.T(), tc.wantAttempt, evt.Data[event.DataKey.AttemptNumber])
			if tc.wantReason != "" {
				assert.Equal(s.T(), string(tc.wantReason), evt.Data[event.DataKey.Error])
			}
			assert.Equal(s.T(), tc.wantCalls, calls.Load())
		})
	}
}

// Many participants fail together, so their retry timers fire together.
func (s *DispatcherTestSuite) TestConcurrentRetries() {
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen[r.URL.Path]++
		first := seen[r.URL.Path] == 1
		mu.Unlock()
		if first {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.MaxInFlight = 64
	s.build(cfg)
	s.d.retryDelay = time.Millisecond
	appIDs := make([]string, 0, 50)
	for i := range 50 {
		id := fmt.Sprintf("app-%d", i)
		appIDs = append(appIDs, id)
		s.withClient(id, srv.URL+"/"+id)
	}
	s.start()

	s.d.OnEvent(context.Background(), terminated(appIDs...))
	events := s.waitFor(len(appIDs))

	for _, evt := range events {
		assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), evt.Type)
		assert.Equal(s.T(), 2, evt.Data[event.DataKey.AttemptNumber])
	}
}

func (s *DispatcherTestSuite) TestUnreachableEndpointExhaustsAttempts() {
	srv := httptest.NewServer(http.NotFoundHandler())
	uri := srv.URL
	srv.Close()

	s.build(testConfig())
	s.withClient("app-1", uri)
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(reasonUnreachable), evt.Data[event.DataKey.Error])
	assert.Equal(s.T(), 3, evt.Data[event.DataKey.AttemptNumber])
}

func (s *DispatcherTestSuite) TestPrivateAddressIsTerminal() {
	srv, calls := s.statusServer(http.StatusOK)
	cfg := testConfig()
	reject := true
	cfg.RejectPrivateAddresses = &reject
	s.build(cfg)
	s.withClient("app-1", srv.URL)
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(reasonPrivateAddress), evt.Data[event.DataKey.Error])
	assert.Equal(s.T(), 1, evt.Data[event.DataKey.AttemptNumber])
	assert.Zero(s.T(), calls.Load())
}

func (s *DispatcherTestSuite) TestSkipsParticipantWithoutURIAndRecordsMissingClient() {
	srv, _ := s.statusServer(http.StatusOK)
	s.build(testConfig())
	s.withClient("no-uri", "")
	s.withClient("app-1", srv.URL)
	// An application that was deleted: no OAuth client and no inbound client either.
	s.clients.On("GetOAuthClientByID", mock.Anything, "gone").
		Return((*providers.OAuthClient)(nil), (*tidcommon.ServiceError)(nil))
	notFound := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ACP-1001"}
	s.clients.On("GetInboundClientByID", mock.Anything, "gone").Return((*providers.InboundClient)(nil), notFound)
	// An application that exists but has no OAuth client, such as one signing in through the Flow API.
	s.clients.On("GetOAuthClientByID", mock.Anything, "no-oauth").
		Return((*providers.OAuthClient)(nil), (*tidcommon.ServiceError)(nil))
	s.clients.On("GetInboundClientByID", mock.Anything, "no-oauth").
		Return(&providers.InboundClient{ID: "no-oauth"}, (*tidcommon.ServiceError)(nil))
	s.start()

	s.d.OnEvent(context.Background(), terminated("no-uri", "gone", "no-oauth", "app-1"))
	s.waitFor(2)
	s.d.Stop()

	byApp := map[string]*providers.Event{}
	for _, evt := range s.snapshot() {
		byApp[evt.Data[event.DataKey.EntityID].(string)] = evt
	}
	require.Len(s.T(), byApp, 2)
	assert.Equal(s.T(), string(reasonClientNotFound), byApp["gone"].Data[event.DataKey.Error])
	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), byApp["app-1"].Type)
}

func (s *DispatcherTestSuite) TestClientReadFailureIsRetried() {
	srv, calls := s.statusServer(http.StatusOK)
	s.build(testConfig())
	client := &providers.OAuthClient{ID: "app-1", ClientID: "client-app-1", BackchannelLogoutURI: srv.URL}
	s.clients.On("GetOAuthClientByID", mock.Anything, "app-1").Return(nil, &tidcommon.InternalServerError).Once()
	s.clients.On("GetOAuthClientByID", mock.Anything, "app-1").Return(client, nil).Once()
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), evt.Type)
	assert.Equal(s.T(), 2, evt.Data[event.DataKey.AttemptNumber])
	assert.Equal(s.T(), int32(1), calls.Load())
}

func (s *DispatcherTestSuite) TestClientReadFailureExhaustsAttempts() {
	s.build(testConfig())
	s.clients.On("GetOAuthClientByID", mock.Anything, "app-1").Return(nil, &tidcommon.InternalServerError)
	s.start()

	var evt *providers.Event
	out := s.captureLogs("INFO", func() {
		s.d.OnEvent(context.Background(), terminated("app-1"))
		evt = s.waitFor(1)[0]
	})

	assert.Equal(s.T(), string(reasonServerError), evt.Data[event.DataKey.Error])
	assert.Equal(s.T(), 3, evt.Data[event.DataKey.AttemptNumber])
	s.clients.AssertNumberOfCalls(s.T(), "GetOAuthClientByID", 3)
	assert.Contains(s.T(), out, tidcommon.InternalServerError.Code, "the warning carries the lookup's cause")
}

func (s *DispatcherTestSuite) TestInboundClientReadFailureIsRetried() {
	srv, calls := s.statusServer(http.StatusOK)
	s.build(testConfig())
	s.clients.On("GetOAuthClientByID", mock.Anything, "app-1").
		Return((*providers.OAuthClient)(nil), (*tidcommon.ServiceError)(nil)).Once()
	s.clients.On("GetInboundClientByID", mock.Anything, "app-1").
		Return((*providers.InboundClient)(nil), &tidcommon.InternalServerError).Once()
	s.withClient("app-1", srv.URL)
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), evt.Type)
	assert.Equal(s.T(), 2, evt.Data[event.DataKey.AttemptNumber])
	assert.Equal(s.T(), int32(1), calls.Load())
}

func (s *DispatcherTestSuite) TestTokenBuildFailureIsRetriedThenRecorded() {
	srv, calls := s.statusServer(http.StatusOK)
	cfg := testConfig()
	s.tokens.On("BuildLogoutToken", mock.Anything, mock.Anything).Return(nil, errors.New("no signing key"))
	s.buildWithTokens(cfg)
	s.withClient("app-1", srv.URL)
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(reasonServerError), evt.Data[event.DataKey.Error])
	assert.Equal(s.T(), 3, evt.Data[event.DataKey.AttemptNumber])
	assert.Zero(s.T(), calls.Load())
	s.tokens.AssertNumberOfCalls(s.T(), "BuildLogoutToken", 3)
}

func (s *DispatcherTestSuite) TestRecoversPanicInAttempt() {
	cfg := testConfig()
	s.tokens.On("BuildLogoutToken", mock.Anything, mock.Anything).Run(func(mock.Arguments) { panic("boom") })
	s.buildWithTokens(cfg)
	s.withClient("app-1", "https://rp.example.com/bcl")
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(reasonServerError), evt.Data[event.DataKey.Error])
}

func (s *DispatcherTestSuite) TestQueueFullDropsWithoutBlocking() {
	cfg := testConfig()
	cfg.QueueSize = 1
	s.build(cfg)
	// Mark the dispatcher started without workers, so nothing drains the queue.
	s.d.started = true
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	defer s.d.cancel()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	s.d.OnEvent(context.Background(), terminated("app-2", "app-3"))

	events := s.waitFor(2)
	require.Len(s.T(), events, 2)
	for _, evt := range events {
		assert.Equal(s.T(), string(reasonQueueFull), evt.Data[event.DataKey.Error])
	}
	assert.Len(s.T(), s.d.intake, 1)
}

// A dropped termination writes one warning for the whole termination but still publishes an event
// for every participant.
func (s *DispatcherTestSuite) TestQueueFullLogsOncePerTermination() {
	cfg := testConfig()
	cfg.QueueSize = 1
	s.build(cfg)
	// Mark the dispatcher started without workers, so nothing drains the queue.
	s.d.started = true
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	defer s.d.cancel()
	s.d.OnEvent(context.Background(), terminated("app-1"))

	out := s.captureLogs("INFO", func() {
		s.d.OnEvent(context.Background(), terminated("app-2", "app-3", "app-4"))
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	s.Require().Len(lines, 1, "one warning for the dropped termination: %s", out)
	for _, want := range []string{"level=WARN", "notifications dropped", "sessionID=session-1", "participants=3",
		"reason=queue_full"} {
		assert.Contains(s.T(), lines[0], want)
	}
	assert.Len(s.T(), s.waitFor(3), 3, "one event per dropped participant")
}

func (s *DispatcherTestSuite) TestEventBeforeStartOrAfterStopIsRecorded() {
	s.build(testConfig())
	s.d.OnEvent(context.Background(), terminated("app-1"))

	s.d.Start(context.Background())
	s.d.Stop()
	s.d.Stop()
	s.d.OnEvent(context.Background(), terminated("app-2"))

	events := s.waitFor(2)
	for _, evt := range events {
		assert.Equal(s.T(), string(reasonShutdown), evt.Data[event.DataKey.Error])
	}
}

func (s *DispatcherTestSuite) TestStopBeforeStartDoesNotPreventLaterStop() {
	srv, _ := s.statusServer(http.StatusOK)
	s.build(testConfig())
	s.withClient("app-1", srv.URL)

	s.d.Stop()
	s.d.Start(context.Background())
	s.d.OnEvent(context.Background(), terminated("app-1"))
	s.d.Stop()

	events := s.snapshot()
	require.Len(s.T(), events, 1)
	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), events[0].Type)
	assert.Error(s.T(), s.d.ctx.Err(), "the second Stop must stop the workers")
}

func (s *DispatcherTestSuite) TestBoundsInFlightDeliveries() {
	var current, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		current.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := testConfig()
	cfg.MaxInFlight = 2
	s.build(cfg)
	appIDs := []string{"a", "b", "c", "d", "e", "f"}
	for _, id := range appIDs {
		s.withClient(id, srv.URL)
	}
	s.start()

	s.d.OnEvent(context.Background(), terminated(appIDs...))
	s.waitFor(len(appIDs))

	assert.LessOrEqual(s.T(), peak.Load(), int32(2))
}

// A relying party whose endpoint hangs holds at most its share of max_in_flight, so a healthy
// relying party in the same terminations is still delivered to promptly (spec R6).
func (s *DispatcherTestSuite) TestHangingClientCannotTakeEveryPermit() {
	hanging, _ := s.hangingServer()
	healthy, healthyCalls := s.statusServer(http.StatusOK)
	cfg := testConfig()
	cfg.MaxInFlight = 2
	s.build(cfg)
	s.withClient("slow", hanging.URL)
	s.withClient("healthy", healthy.URL)
	s.start()
	s.Require().Equal(1, s.d.clientLimit)

	for range 3 {
		s.d.OnEvent(context.Background(), terminated("slow", "healthy"))
	}

	s.Require().Eventually(func() bool { return healthyCalls.Load() == 3 }, 2*time.Second, 5*time.Millisecond,
		"the healthy relying party must not wait behind the hanging one")
	s.d.mu.RLock()
	defer s.d.mu.RUnlock()
	assert.LessOrEqual(s.T(), s.d.clientInFlight["slow"], 1)
}

// A job turned away because its relying party holds its share waits without using an attempt.
func (s *DispatcherTestSuite) TestJobOverClientShareKeepsItsAttempt() {
	s.build(testConfig())
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	defer s.d.cancel()
	s.d.clientInFlight["app-1"] = s.d.clientLimit
	j := &job{sessionID: "session-1", appID: "app-1", attempt: 1}

	s.d.launch(j)

	s.d.mu.RLock()
	defer s.d.mu.RUnlock()
	assert.Contains(s.T(), s.d.pending, j, "the job waits on a timer")
	assert.Equal(s.T(), 1, j.attempt)
	assert.Empty(s.T(), s.d.inFlight, "no global permit is taken")
	s.d.pending[j].Stop()
}

func (s *DispatcherTestSuite) TestStopDrainsInFlightDelivery() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s.build(testConfig())
	s.withClient("app-1", srv.URL)
	s.d.Start(context.Background())

	s.d.OnEvent(context.Background(), terminated("app-1"))
	s.d.Stop()

	events := s.snapshot()
	require.Len(s.T(), events, 1)
	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), events[0].Type)
}

func (s *DispatcherTestSuite) TestStopAbandonsAtDeadline() {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	cfg := testConfig()
	s.build(cfg)
	s.d.shutdownBudget = 100 * time.Millisecond
	s.withClient("app-1", srv.URL)
	s.d.Start(context.Background())

	s.d.OnEvent(context.Background(), terminated("app-1"))
	require.Eventually(s.T(), func() bool { return len(s.d.inFlight) == 1 }, time.Second, 5*time.Millisecond)

	start := time.Now()
	s.d.Stop()

	assert.Less(s.T(), time.Since(start), time.Second)
	events := s.waitFor(1)
	assert.Equal(s.T(), string(reasonShutdown), events[0].Data[event.DataKey.Error])
}

func (s *DispatcherTestSuite) TestStopRecordsPendingRetry() {
	srv, calls := s.statusServer(http.StatusServiceUnavailable)
	cfg := testConfig()
	s.build(cfg)
	s.d.retryDelay = time.Hour
	s.withClient("app-1", srv.URL)
	s.d.Start(context.Background())

	s.d.OnEvent(context.Background(), terminated("app-1"))
	require.Eventually(s.T(), func() bool {
		s.d.mu.RLock()
		defer s.d.mu.RUnlock()
		return len(s.d.pending) == 1
	}, time.Second, 5*time.Millisecond)

	s.d.Stop()

	events := s.snapshot()
	require.Len(s.T(), events, 1)
	assert.Equal(s.T(), string(reasonShutdown), events[0].Data[event.DataKey.Error])
	assert.Equal(s.T(), 2, events[0].Data[event.DataKey.AttemptNumber])
	assert.Equal(s.T(), int32(1), calls.Load())
}

// AC6.3: every attempt at an endpoint that never answers is cut off by request_timeout, and the
// delivery settles after max_attempts.
func (s *DispatcherTestSuite) TestTimeoutBoundsEachAttempt() {
	srv, calls := s.hangingServer()
	s.tokens.On("BuildLogoutToken", mock.Anything, mock.Anything).
		Return(&oauth2model.TokenDTO{Token: "logout-token"}, nil)
	s.d = newDispatcher(testConfig(), s.tokens, s.clients,
		syshttp.NewHTTPClient(syshttp.HTTPClientConfig{
			Timeout:          50 * time.Millisecond,
			DisableRedirects: true,
		}), s.events)
	s.d.retryDelay = time.Millisecond
	s.withClient("app-1", srv.URL)
	s.start()

	// The server never answers, so reaching the third attempt and settling as unreachable is only
	// possible if the client's timeout cut off each attempt; without it, waitFor would time out.
	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(reasonUnreachable), evt.Data[event.DataKey.Error])
	assert.Equal(s.T(), 3, evt.Data[event.DataKey.AttemptNumber])
	assert.Equal(s.T(), int32(3), calls.Load())
}

// AC6.6: the response body is drained to a fixed cap and never parsed; only the status code counts.
func (s *DispatcherTestSuite) TestResponseBodyIsDrainedToCapAndIgnored() {
	body := &endlessBody{}
	httpClient := httpmock.NewHTTPClientInterfaceMock(s.T())
	httpClient.On("Do", mock.Anything).Return(&http.Response{StatusCode: http.StatusOK, Body: body}, nil)
	s.tokens.On("BuildLogoutToken", mock.Anything, mock.Anything).
		Return(&oauth2model.TokenDTO{Token: "logout-token"}, nil)
	s.d = newDispatcher(testConfig(), s.tokens, s.clients, httpClient, s.events)
	s.withClient("app-1", "https://rp.example.com/bcl")
	s.start()

	s.d.OnEvent(context.Background(), terminated("app-1"))
	evt := s.waitFor(1)[0]

	assert.Equal(s.T(), string(event.EventTypeBackchannelLogoutDelivered), evt.Type)
	assert.LessOrEqual(s.T(), body.read.Load(), int64(responseDrainLimit))
	assert.True(s.T(), body.closed.Load(), "the response body must be closed")
}

// AC7.3: a delivery that fails writes exactly one warning carrying the recorded fields, and a
// delivery that succeeds writes nothing at the default level.
func (s *DispatcherTestSuite) TestLogsOneWarningPerFailure() {
	rejecting, _ := s.statusServer(http.StatusBadRequest)
	accepting, _ := s.statusServer(http.StatusOK)
	s.build(testConfig())
	s.withClient("app-1", rejecting.URL)
	s.withClient("app-2", accepting.URL)
	s.start()

	out := s.captureLogs("INFO", func() {
		s.d.OnEvent(context.Background(), terminated("app-1", "app-2"))
		s.waitFor(2)
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	s.Require().Len(lines, 1, "only the failure is logged: %s", out)
	for _, want := range []string{"level=WARN", "Back-channel logout delivery failed", "clientID=client-app-1",
		"appID=app-1", "sessionID=session-1", "attempt=1", "statusCode=400", "reason=rejected"} {
		assert.Contains(s.T(), lines[0], want)
	}
}

// AC7.4: neither events nor logs, at any level, carry the token, the URI query, a body, or the subject.
func (s *DispatcherTestSuite) TestRecordsCarryNoSecrets() {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_, _ = w.Write([]byte("body-secret"))
	}))
	defer srv.Close()
	s.build(testConfig())
	s.withClient("app-1", srv.URL+"/bcl?k=query-secret")
	s.start()

	out := s.captureLogs("DEBUG", func() {
		s.d.OnEvent(context.Background(), terminated("app-1"))
		s.waitFor(1)
	})

	recorded := out + fmt.Sprint(s.snapshot()[0].Data)
	s.Require().Contains(out, "retrying", "the debug retry line must be part of what is checked")
	for _, secret := range []string{"logout-token", "query-secret", "body-secret", "user-1"} {
		assert.NotContains(s.T(), recorded, secret)
	}
}

// AC7.4 for a connection error, whose message would otherwise repeat the URL and its query.
func (s *DispatcherTestSuite) TestConnectionErrorLogsNoURI() {
	srv := httptest.NewServer(http.NotFoundHandler())
	uri := srv.URL + "/bcl?k=query-secret"
	srv.Close()
	s.build(testConfig())
	s.withClient("app-1", uri)
	s.d.retryDelay = time.Millisecond
	s.start()

	out := s.captureLogs("DEBUG", func() {
		s.d.OnEvent(context.Background(), terminated("app-1"))
		s.waitFor(1)
	})

	s.Require().Contains(out, "connection refused", "the cause is still logged")
	assert.NotContains(s.T(), out, "query-secret")
	assert.NotContains(s.T(), out, "/bcl")
}

// Work still queued when the shutdown deadline passes is recorded as shutdown, not lost.
func (s *DispatcherTestSuite) TestStopRecordsQueuedEventsAtDeadline() {
	srv, _ := s.hangingServer()
	cfg := testConfig()
	cfg.MaxInFlight = 1
	s.build(cfg)
	s.d.workerCount = 1
	s.d.shutdownBudget = 100 * time.Millisecond
	for _, id := range []string{"app-1", "app-2", "app-3"} {
		s.withClient(id, srv.URL)
	}
	s.d.Start(context.Background())

	// app-1 holds the only permit, the worker waits for it with app-2, and app-3 stays queued.
	s.d.OnEvent(context.Background(), terminated("app-1"))
	s.Require().Eventually(func() bool { return len(s.d.inFlight) == 1 }, time.Second, 5*time.Millisecond)
	s.d.OnEvent(context.Background(), terminated("app-2"))
	s.Require().Eventually(func() bool { return len(s.d.intake) == 0 }, time.Second, 5*time.Millisecond)
	s.d.OnEvent(context.Background(), terminated("app-3"))
	s.Require().Len(s.d.intake, 1)

	s.d.Stop()

	events := s.waitFor(3)
	byApp := map[string]any{}
	for _, evt := range events {
		byApp[evt.Data[event.DataKey.EntityID].(string)] = evt.Data[event.DataKey.Error]
	}
	for _, id := range []string{"app-1", "app-2", "app-3"} {
		assert.Equal(s.T(), string(reasonShutdown), byApp[id], "%s should be recorded as shutdown", id)
	}
	assert.Empty(s.T(), s.d.intake)
}

// The shutdown paths below are exercised directly rather than through timing, so each runs on
// every test run.

// abandonQueued records whatever is left on both queues once the workers are gone.
func (s *DispatcherTestSuite) TestAbandonQueuedRecordsBothQueues() {
	s.build(testConfig())
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	s.d.cancel()
	s.d.outstanding.Add(2)
	s.d.intake <- termination{sessionID: "session-1", appIDs: []string{"app-1"}}
	s.d.retries <- &job{sessionID: "session-1", appID: "app-2", attempt: 2}

	s.d.abandonQueued()

	events := s.snapshot()
	s.Require().Len(events, 2)
	reasons := map[string]any{}
	for _, evt := range events {
		reasons[evt.Data[event.DataKey.EntityID].(string)] = evt.Data[event.DataKey.Error]
	}
	assert.Equal(s.T(), map[string]any{"app-1": string(reasonShutdown), "app-2": string(reasonShutdown)}, reasons)
	assert.Empty(s.T(), s.d.intake)
	assert.Empty(s.T(), s.d.retries)
	assert.True(s.T(), s.d.waitIdle(time.Second), "every abandoned item is accounted for")
}

// A retry timer that fires once Stop has begun records the job instead of handing it back.
func (s *DispatcherTestSuite) TestRetryAfterStopRecordsShutdown() {
	s.build(testConfig())
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	defer s.d.cancel()
	s.d.stopping = true
	j := &job{sessionID: "session-1", appID: "app-1", attempt: 2}
	s.d.pending[j] = nil
	s.d.outstanding.Add(1)

	s.d.retry(j)

	events := s.snapshot()
	s.Require().Len(events, 1)
	assert.Equal(s.T(), string(reasonShutdown), events[0].Data[event.DataKey.Error])
	assert.Equal(s.T(), 2, events[0].Data[event.DataKey.AttemptNumber])
	assert.NotContains(s.T(), s.d.pending, j)
	assert.Empty(s.T(), s.d.retries)
	assert.True(s.T(), s.d.waitIdle(time.Second))
}

// A retry timer that cannot hand its job back because the queue is full records it once the
// dispatcher is canceled, and no longer counts as requeueing.
func (s *DispatcherTestSuite) TestRetryWithFullQueueRecordsShutdownOnCancel() {
	cfg := testConfig()
	cfg.QueueSize = 1
	s.build(cfg)
	s.d.ctx, s.d.cancel = context.WithCancel(context.Background())
	s.d.retries <- &job{appID: "queued"}
	j := &job{sessionID: "session-1", appID: "app-1", attempt: 2}
	s.d.outstanding.Add(1)

	done := make(chan struct{})
	go func() {
		s.d.retry(j)
		close(done)
	}()
	s.d.cancel()
	s.Require().Eventually(func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, time.Second, 5*time.Millisecond)

	events := s.snapshot()
	s.Require().Len(events, 1)
	assert.Equal(s.T(), "app-1", events[0].Data[event.DataKey.EntityID])
	assert.Equal(s.T(), string(reasonShutdown), events[0].Data[event.DataKey.Error])
	s.d.requeueing.Wait()
	assert.True(s.T(), s.d.waitIdle(time.Second))
}

// A retry scheduled once Stop has begun is recorded at once, with the attempt it would have made.
func (s *DispatcherTestSuite) TestScheduleAfterStopRecordsShutdown() {
	s.build(testConfig())
	s.d.stopping = true
	j := &job{sessionID: "session-1", appID: "app-1", attempt: 1}

	s.d.schedule(j, time.Hour)

	events := s.snapshot()
	s.Require().Len(events, 1)
	assert.Equal(s.T(), string(reasonShutdown), events[0].Data[event.DataKey.Error])
	assert.Equal(s.T(), 2, events[0].Data[event.DataKey.AttemptNumber])
	assert.Empty(s.T(), s.d.pending)
	assert.True(s.T(), s.d.waitIdle(time.Second), "nothing was left outstanding")
}

func (s *DispatcherTestSuite) TestNextDelay() {
	d := &dispatcher{cfg: engineconfig.BackchannelLogoutConfig{MaxAttempts: 3}, retryDelay: 2 * time.Second}

	assert.Equal(s.T(), 2*time.Second, d.nextDelay(1, 0))
	assert.Equal(s.T(), 4*time.Second, d.nextDelay(2, 0))
	assert.Equal(s.T(), time.Second, d.nextDelay(1, time.Second), "Retry-After below the bound is honored")
	assert.Equal(s.T(), 4*time.Second, d.nextDelay(1, time.Hour), "Retry-After is bounded by the longest wait")
}

// At the largest settings validation allows, the doubled wait stays positive and bounds Retry-After.
func (s *DispatcherTestSuite) TestNextDelayAtConfigBounds() {
	d := newDispatcher(engineconfig.BackchannelLogoutConfig{
		RequestTimeout: 30, MaxAttempts: 10, RetryDelay: 60, MaxInFlight: 1, QueueSize: 1,
	}, s.tokens, s.clients, nil, s.events)

	assert.Equal(s.T(), 60*time.Second<<8, d.nextDelay(9, 0), "the last wait of the schedule")
	assert.Equal(s.T(), 60*time.Second<<8, d.nextDelay(1, 1000*time.Hour), "Retry-After is bounded by it")
}

func (s *DispatcherTestSuite) TestParseRetryAfter() {
	assert.Equal(s.T(), 3*time.Second, parseRetryAfter("3"))
	assert.Equal(s.T(), 3*time.Second, parseRetryAfter(" 3 "))
	assert.Zero(s.T(), parseRetryAfter(""))
	assert.Zero(s.T(), parseRetryAfter("0"))
	assert.Zero(s.T(), parseRetryAfter("-1"))
	assert.Zero(s.T(), parseRetryAfter("soon"))

	future := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	assert.InDelta(s.T(), time.Hour, parseRetryAfter(future), float64(2*time.Second),
		"an HTTP date is a delay until it")
	assert.Zero(s.T(), parseRetryAfter(past), "a past date is absent")
}

// The dispatcher takes the deployment configuration as it is, reading its seconds as durations.
func (s *DispatcherTestSuite) TestNewDispatcherReadsDeploymentConfig() {
	d := newDispatcher(engineconfig.BackchannelLogoutConfig{
		RequestTimeout: 5,
		MaxAttempts:    3,
		RetryDelay:     2,
		MaxInFlight:    16,
		QueueSize:      1024,
	}, s.tokens, s.clients, nil, s.events)

	assert.Equal(s.T(), 2*time.Second, d.retryDelay)
	assert.Equal(s.T(), defaultWorkers, d.workerCount)
	assert.Equal(s.T(), defaultShutdownBudget, d.shutdownBudget)
	assert.Equal(s.T(), 1024, cap(d.intake))
	assert.Equal(s.T(), 16, cap(d.inFlight))
	assert.Equal(s.T(), 4, d.clientLimit, "a quarter of max_in_flight")
}
