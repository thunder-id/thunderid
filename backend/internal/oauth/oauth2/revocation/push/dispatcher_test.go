// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// plainClient sends requests with the standard client; the dispatcher only uses Do.
type plainClient struct {
	syshttp.HTTPClientInterface
	client *http.Client
}

func (c plainClient) Do(req *http.Request) (*http.Response, error) { return c.client.Do(req) }

type receivedPush struct {
	contentType string
	body        string
}

// receiver is a test endpoint that records each push and answers with the next scripted status.
type receiver struct {
	server   *httptest.Server
	received chan receivedPush
	statuses []int
	calls    atomic.Int32
}

func newReceiver(t *testing.T, statuses ...int) *receiver {
	r := &receiver{received: make(chan receivedPush, 16), statuses: statuses}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.received <- receivedPush{contentType: req.Header.Get(serverconst.ContentTypeHeaderName), body: string(body)}
		call := int(r.calls.Add(1)) - 1
		status := r.statuses[min(call, len(r.statuses)-1)]
		w.WriteHeader(status)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *receiver) next(t *testing.T) receivedPush {
	t.Helper()
	select {
	case p := <-r.received:
		return p
	case <-time.After(2 * time.Second):
		require.Fail(t, "no push received")
		return receivedPush{}
	}
}

func (r *receiver) none(t *testing.T) {
	t.Helper()
	select {
	case <-r.received:
		require.Fail(t, "unexpected push")
	case <-time.After(150 * time.Millisecond):
	}
}

func testConfig(receivers ...engineconfig.TokenRevocationReceiver) engineconfig.TokenRevocationPushConfig {
	return engineconfig.TokenRevocationPushConfig{
		Receivers: receivers, RequestTimeout: 2, MaxAttempts: 3, RetryDelay: 1, QueueSize: 8,
	}
}

func receiverFor(r *receiver, audience string) engineconfig.TokenRevocationReceiver {
	return engineconfig.TokenRevocationReceiver{URL: r.server.URL, Audience: audience}
}

func newTestDispatcher(t *testing.T, cfg engineconfig.TokenRevocationPushConfig) (*dispatcher,
	*jwtmock.JWTServiceInterfaceMock) {
	jwtService := jwtmock.NewJWTServiceInterfaceMock(t)
	d := newDispatcher(cfg, jwtService, plainClient{client: &http.Client{Timeout: 2 * time.Second}})
	d.retryDelay = 10 * time.Millisecond
	return d, jwtService
}

func expectToken(jwtService *jwtmock.JWTServiceInterfaceMock, audience, token string) *mock.Call {
	return jwtService.On("GenerateJWT", mock.Anything, "", "", int64(eventValidityPeriod),
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEvents := claims["events"]
			return hasEvents && claims["aud"] == audience
		}), sharedrevocation.SecurityEventTokenType, "").
		Return(token, int64(0), (*tidcommon.ServiceError)(nil))
}

func subjectEvent() sharedrevocation.Event {
	return sharedrevocation.Event{
		Criterion: sharedrevocation.Criterion{Type: sharedrevocation.CriterionTypeSubject, Value: "user-1"},
		Reason:    sharedrevocation.ReasonUserDeleted, RevokedAt: time.Now(), ExpiryTime: time.Now().Add(time.Hour),
	}
}

func TestDispatcher_DeliversASignedEventToEveryReceiver(t *testing.T) {
	first, second := newReceiver(t, http.StatusAccepted), newReceiver(t, http.StatusAccepted)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(first, "aud-1"), receiverFor(second, "aud-2")))
	expectToken(jwtService, "aud-1", "token-1")
	expectToken(jwtService, "aud-2", "token-2")
	d.Start(context.Background())
	defer d.Stop()

	d.OnEvent(context.Background(), subjectEvent())

	got := first.next(t)
	assert.Equal(t, serverconst.ContentTypeSecurityEventToken, got.contentType)
	assert.Equal(t, "token-1", got.body)
	assert.Equal(t, "token-2", second.next(t).body, "each receiver is addressed by its own audience")
}

func TestDispatcher_RetriesATransientFailure(t *testing.T) {
	r := newReceiver(t, http.StatusServiceUnavailable, http.StatusTooManyRequests, http.StatusAccepted)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	expectToken(jwtService, "aud", "token")
	d.Start(context.Background())
	defer d.Stop()

	d.OnEvent(context.Background(), subjectEvent())

	for range 3 {
		assert.Equal(t, "token", r.next(t).body)
	}
	r.none(t)
}

func TestDispatcher_StopsAfterMaxAttempts(t *testing.T) {
	r := newReceiver(t, http.StatusInternalServerError)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	expectToken(jwtService, "aud", "token")
	d.Start(context.Background())
	defer d.Stop()

	d.OnEvent(context.Background(), subjectEvent())

	for range 3 {
		r.next(t)
	}
	r.none(t)
}

func TestDispatcher_DoesNotRetryARejection(t *testing.T) {
	r := newReceiver(t, http.StatusBadRequest)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	expectToken(jwtService, "aud", "token")
	d.Start(context.Background())
	defer d.Stop()

	d.OnEvent(context.Background(), subjectEvent())

	r.next(t)
	r.none(t)
}

func TestDispatcher_RetriesWhenTheTokenCannotBeBuilt(t *testing.T) {
	r := newReceiver(t, http.StatusAccepted)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	jwtService.On("GenerateJWT", mock.Anything, "", "", int64(eventValidityPeriod), mock.Anything,
		sharedrevocation.SecurityEventTokenType, "").Return("", int64(0), &tidcommon.InternalServerError).Once()
	expectToken(jwtService, "aud", "token")
	d.Start(context.Background())
	defer d.Stop()

	d.OnEvent(context.Background(), subjectEvent())

	assert.Equal(t, "token", r.next(t).body)
}

func TestDispatcher_DropsEventsBeforeStartAndAfterStop(t *testing.T) {
	r := newReceiver(t, http.StatusAccepted)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	expectToken(jwtService, "aud", "token").Maybe()

	d.OnEvent(context.Background(), subjectEvent())
	d.Start(context.Background())
	d.Stop()
	d.Stop()
	d.OnEvent(context.Background(), subjectEvent())

	r.none(t)
}

func TestDispatcher_DropsEventsWhenTheQueueIsFull(t *testing.T) {
	cfg := testConfig(engineconfig.TokenRevocationReceiver{URL: "https://receiver.example.com", Audience: "aud"})
	cfg.QueueSize = 1
	d, _ := newTestDispatcher(t, cfg)
	d.started = true // workers are not running, so the queue only fills

	d.OnEvent(context.Background(), subjectEvent())
	d.OnEvent(context.Background(), subjectEvent())

	assert.Len(t, d.queue, 1)
}

func TestDispatcher_StopDropsPendingRetries(t *testing.T) {
	r := newReceiver(t, http.StatusInternalServerError)
	d, jwtService := newTestDispatcher(t, testConfig(receiverFor(r, "aud")))
	d.retryDelay = time.Hour
	expectToken(jwtService, "aud", "token")
	d.Start(context.Background())

	d.OnEvent(context.Background(), subjectEvent())
	r.next(t)
	require.Eventually(t, func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return len(d.pending) == 1
	}, 2*time.Second, 10*time.Millisecond)

	d.Stop()

	assert.Empty(t, d.pending)
	r.none(t)
}
