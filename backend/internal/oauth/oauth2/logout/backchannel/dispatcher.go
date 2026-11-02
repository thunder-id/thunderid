// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package backchannel delivers OIDC Back-Channel Logout notifications. Its dispatcher listens for
// terminated SSO sessions and POSTs a logout token to every participant that registered a
// back-channel logout URI, in process, asynchronously, with bounded retry and a shutdown drain.
package backchannel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DispatcherInterface turns terminated sessions into logout token deliveries. OnEvent copies the
// event onto a bounded queue and returns; workers resolve each participant, build a token per
// attempt, POST it, retry transient failures, and record one terminal outcome per participant.
type DispatcherInterface interface {
	eventlistener.Listener[session.TerminatedSession]
	// Start launches the workers. It is not called on the bootstrap path.
	Start(ctx context.Context)
	// Stop refuses new events, lets queued and in-flight deliveries finish within the shutdown
	// budget, and records anything abandoned as shutdown. It is safe to call more than once.
	Stop()
}

// dispatcher is the DispatcherInterface implementation: a bounded intake queue, a worker pool, a
// retry scheduler and an in-flight bound, all in process.
type dispatcher struct {
	cfg engineconfig.BackchannelLogoutConfig
	// retryDelay is cfg's whole seconds as a duration, so tests can shorten it.
	retryDelay time.Duration
	// workerCount and shutdownBudget are not deployment settings.
	workerCount    int
	shutdownBudget time.Duration
	// clientLimit is the most permits one relying party may hold: its share of max_in_flight.
	clientLimit int

	tokens  tokenservice.TokenBuilderInterface
	clients providers.ActorProvider
	http    syshttp.HTTPClientInterface
	events  providers.ObservabilityProvider
	logger  *log.Logger

	intake   chan termination
	retries  chan *job
	inFlight chan struct{}

	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
	// outstanding counts queued terminations, running attempts and scheduled retries, so Stop knows
	// when the dispatcher is idle.
	outstanding sync.WaitGroup
	// requeueing counts retry timers handing a job back to the workers, so Stop can drain the retry
	// queue only once none of them can still add to it.
	requeueing sync.WaitGroup

	mu       sync.RWMutex
	started  bool
	stopping bool
	pending  map[*job]*time.Timer
	// clientInFlight counts the permits each relying party holds, keyed by application id.
	clientInFlight map[string]int
}

// newDispatcher creates a stopped dispatcher from the deployment configuration, which is validated
// at startup, so every count and duration is positive. events may be nil.
func newDispatcher(cfg engineconfig.BackchannelLogoutConfig, tokens tokenservice.TokenBuilderInterface,
	clients providers.ActorProvider, httpClient syshttp.HTTPClientInterface,
	events providers.ObservabilityProvider) *dispatcher {
	return &dispatcher{
		cfg:            cfg,
		retryDelay:     time.Duration(cfg.RetryDelay) * time.Second,
		workerCount:    defaultWorkers,
		shutdownBudget: defaultShutdownBudget,
		clientLimit:    max(1, cfg.MaxInFlight/clientShareDivisor),
		clientInFlight: make(map[string]int),
		tokens:         tokens,
		clients:        clients,
		http:           httpClient,
		events:         events,
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "BackchannelLogoutDispatcher")),
		intake:         make(chan termination, cfg.QueueSize),
		retries:        make(chan *job, cfg.QueueSize),
		inFlight:       make(chan struct{}, cfg.MaxInFlight),
		pending:        make(map[*job]*time.Timer),
	}
}

// Start implements DispatcherInterface.
func (d *dispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.started {
		return
	}
	d.started = true
	d.ctx, d.cancel = context.WithCancel(context.WithoutCancel(ctx))
	for range d.workerCount {
		d.workers.Add(1)
		go d.work()
	}
}

// OnEvent implements eventlistener.Listener. It never blocks.
func (d *dispatcher) OnEvent(ctx context.Context, ended session.TerminatedSession) {
	t := termination{
		traceID:   syscontext.GetTraceID(ctx),
		sessionID: ended.SessionID,
		subjectID: ended.SubjectID,
		appIDs:    make([]string, 0, len(ended.Participants)),
	}
	for _, p := range ended.Participants {
		t.appIDs = append(t.appIDs, p.AppID)
	}

	// Decide under the lock, record after it: recording writes a log line, which must not hold up
	// Stop or other requests.
	d.mu.RLock()
	reason, accepted := reasonShutdown, false
	if d.started && !d.stopping {
		d.outstanding.Add(1)
		select {
		case d.intake <- t:
			accepted = true
		default:
			d.outstanding.Done()
			reason = reasonQueueFull
		}
	}
	d.mu.RUnlock()
	if !accepted {
		d.recordDropped(t, reason)
	}
}

// Stop implements DispatcherInterface. A Stop before Start does nothing, so a later Start and Stop
// still work.
func (d *dispatcher) Stop() {
	d.mu.Lock()
	if !d.started || d.stopping {
		d.mu.Unlock()
		return
	}
	d.stopping = true
	// A retry whose timer has not fired will not be attempted again. Collect them under the lock and
	// record them after it, so the lock is not held for one log line per job.
	var parked []*job
	for j, timer := range d.pending {
		if timer.Stop() {
			delete(d.pending, j)
			parked = append(parked, j)
		}
	}
	d.mu.Unlock()
	for _, j := range parked {
		d.record(j, reasonShutdown, 0, 0, nil)
		d.outstanding.Done()
	}

	if !d.waitIdle(d.shutdownBudget) {
		d.cancel()
		d.workers.Wait()
		d.abandonQueued()
		d.waitIdle(shutdownGrace)
	}
	d.cancel()
	d.workers.Wait()
}

// waitIdle reports whether every outstanding piece of work finished within the timeout.
func (d *dispatcher) waitIdle(timeout time.Duration) bool {
	idle := make(chan struct{})
	go func() {
		d.outstanding.Wait()
		close(idle)
	}()
	select {
	case <-idle:
		return true
	case <-time.After(timeout):
		return false
	}
}

// abandonQueued records whatever the deadline left on the queues. It runs after cancel, once every
// retry timer that was handing a job back has finished, so nothing can enqueue behind it.
func (d *dispatcher) abandonQueued() {
	d.requeueing.Wait()
	for {
		select {
		case t := <-d.intake:
			d.recordDropped(t, reasonShutdown)
			d.outstanding.Done()
		case j := <-d.retries:
			d.record(j, reasonShutdown, 0, 0, nil)
			d.outstanding.Done()
		default:
			return
		}
	}
}

// work is one worker: it expands terminations into jobs and runs retries when they fall due.
func (d *dispatcher) work() {
	defer d.workers.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case t := <-d.intake:
			d.expand(t)
			d.outstanding.Done()
		case j := <-d.retries:
			d.launch(j)
			d.outstanding.Done()
		}
	}
}

// expand resolves each participant and launches a delivery for those with a registered URI.
func (d *dispatcher) expand(t termination) {
	for _, appID := range t.appIDs {
		j := &job{traceID: t.traceID, sessionID: t.sessionID, subjectID: t.subjectID, appID: appID, attempt: 1}
		if d.resolve(j) {
			d.launch(j)
		}
	}
}

// resolve looks up the participant's client and reports whether there is anything to deliver. A
// failed read is retried like a failed attempt; only an application that no longer exists is
// terminal. One that exists but has no OAuth client, such as an app that signs in through the
// Flow API, cannot register a URI and is skipped like one that registered none.
func (d *dispatcher) resolve(j *job) bool {
	ctx := d.workCtx(j.traceID)
	client, svcErr := d.clients.GetOAuthClientByID(ctx, j.appID)
	if svcErr != nil {
		d.settle(j, outcome{retryable: true, reason: reasonServerError, err: readError(svcErr)}, time.Now())
		return false
	}
	if client == nil {
		inbound, inboundErr := d.clients.GetInboundClientByID(ctx, j.appID)
		switch {
		case inboundErr != nil && inboundErr.Type == tidcommon.ServerErrorType:
			d.settle(j, outcome{retryable: true, reason: reasonServerError, err: readError(inboundErr)}, time.Now())
		case inboundErr != nil || inbound == nil:
			d.record(j, reasonClientNotFound, 0, 0, nil)
		}
		return false
	}
	if client.BackchannelLogoutURI == "" {
		return false
	}
	j.client = client
	return true
}

// readError carries a failed lookup's code into the log line; the actor provider logs the detail.
func readError(svcErr *tidcommon.ServiceError) error {
	return fmt.Errorf("failed to read the participant's client: %s", svcErr.Code)
}

// launch runs one attempt on its own goroutine once an in-flight permit is available.
// A relying party already holding its share of permits does not get another: the job waits on a
// timer instead, without using an attempt or holding the worker, so one slow endpoint cannot take
// every permit.
func (d *dispatcher) launch(j *job) {
	if !d.reserveClient(j.appID) {
		d.park(j, d.retryDelay)
		return
	}
	select {
	case d.inFlight <- struct{}{}:
	case <-d.ctx.Done():
		d.releaseClient(j.appID)
		d.record(j, reasonShutdown, 0, 0, nil)
		return
	}
	d.outstanding.Add(1)
	go func() {
		defer d.outstanding.Done()
		defer d.releaseClient(j.appID)
		defer func() { <-d.inFlight }()
		defer func() {
			if r := recover(); r != nil {
				d.record(j, reasonServerError, 0, 0, fmt.Errorf("delivery panicked: %v", r))
			}
		}()
		d.attempt(j)
	}()
}

// attempt builds a fresh token, POSTs it, and settles the job or schedules the next attempt.
func (d *dispatcher) attempt(j *job) {
	// A job whose client lookup failed comes back without a client; look it up again first.
	if j.client == nil && !d.resolve(j) {
		return
	}
	ctx := d.workCtx(j.traceID)
	start := time.Now()
	token, err := d.tokens.BuildLogoutToken(ctx, &tokenservice.LogoutTokenBuildContext{
		OAuthApp:  j.client,
		SubjectID: j.subjectID,
		SessionID: j.sessionID,
	})
	if err != nil {
		d.settle(j, outcome{retryable: true, reason: reasonServerError,
			err: fmt.Errorf("failed to build logout token: %w", err)}, start)
		return
	}
	d.settle(j, d.post(ctx, j.client.BackchannelLogoutURI, token.Token), start)
}

// post performs one delivery request and classifies the response.
func (d *dispatcher) post(ctx context.Context, uri, token string) outcome {
	// The HTTP client bounds the attempt with request_timeout; the context cancels it on shutdown.
	body := url.Values{formParamLogoutToken: {token}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uri, strings.NewReader(body))
	if err != nil {
		return outcome{reason: reasonRejected, err: err}
	}
	req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeFormURLEncoded)

	resp, err := d.http.Do(req)
	if err != nil {
		// A url.Error repeats the request URL, query included; keep only the cause it wraps.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		switch {
		case errors.Is(err, syshttp.ErrPrivateAddress):
			return outcome{reason: reasonPrivateAddress, err: err}
		case d.ctx.Err() != nil:
			return outcome{reason: reasonShutdown}
		default:
			return outcome{retryable: true, reason: reasonUnreachable, err: err}
		}
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, responseDrainLimit))
		_ = resp.Body.Close()
	}()

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent:
		return outcome{delivered: true, status: resp.StatusCode}
	case resp.StatusCode == http.StatusTooManyRequests:
		return outcome{retryable: true, reason: reasonServerError, status: resp.StatusCode,
			retryAfter: parseRetryAfter(resp.Header.Get(serverconst.RetryAfterHeaderName))}
	case resp.StatusCode >= http.StatusInternalServerError:
		return outcome{retryable: true, reason: reasonServerError, status: resp.StatusCode}
	default:
		return outcome{reason: reasonRejected, status: resp.StatusCode}
	}
}

// settle records a terminal outcome or schedules the next attempt.
func (d *dispatcher) settle(j *job, o outcome, start time.Time) {
	elapsed := time.Since(start)
	if o.delivered {
		d.recordDelivered(j, o.status, elapsed)
		return
	}
	if !o.retryable || j.attempt >= d.cfg.MaxAttempts {
		d.record(j, o.reason, o.status, elapsed, o.err)
		return
	}
	delay := d.nextDelay(j.attempt, o.retryAfter)
	if d.logger.IsDebugEnabled() {
		d.logger.Debug(d.recordCtx(j.traceID), "Back-channel logout attempt failed, retrying",
			append(d.failureFields(j, o.reason, o.status, elapsed, o.err),
				log.Int("retryInMs", int(delay.Milliseconds())))...)
	}
	d.schedule(j, delay)
}

// nextDelay doubles the retry delay per attempt. A Retry-After replaces it, bounded by the longest
// wait of the schedule, so a relying party cannot hold a delivery longer than the backoff would.
func (d *dispatcher) nextDelay(attempt int, retryAfter time.Duration) time.Duration {
	delay := d.retryDelay << (attempt - 1)
	if retryAfter > 0 {
		longest := d.retryDelay << (d.cfg.MaxAttempts - 2)
		delay = min(retryAfter, longest)
	}
	return delay
}

// reserveClient takes one of the relying party's permits, reporting false when it holds its share.
func (d *dispatcher) reserveClient(appID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.clientInFlight[appID] >= d.clientLimit {
		return false
	}
	d.clientInFlight[appID]++
	return true
}

// releaseClient returns a permit taken by reserveClient.
func (d *dispatcher) releaseClient(appID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.clientInFlight[appID]--; d.clientInFlight[appID] <= 0 {
		delete(d.clientInFlight, appID)
	}
}

// schedule parks the job on a timer, not a worker, until its next attempt is due.
func (d *dispatcher) schedule(j *job, delay time.Duration) {
	j.attempt++
	d.park(j, delay)
}

// park holds the job on a timer until it is handed back to the workers, or records it once Stop has
// begun.
func (d *dispatcher) park(j *job, delay time.Duration) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		d.record(j, reasonShutdown, 0, 0, nil)
		return
	}
	d.outstanding.Add(1)
	d.pending[j] = time.AfterFunc(delay, func() { d.retry(j) })
	d.mu.Unlock()
}

// retry hands a job whose wait is over back to the workers, or records it once Stop has begun. The
// lock covers only the map and the stopping check: holding it across the send would block an
// attempt that calls schedule while the retry queue is full.
func (d *dispatcher) retry(j *job) {
	d.mu.Lock()
	delete(d.pending, j)
	stopping := d.stopping
	if !stopping {
		d.requeueing.Add(1)
	}
	d.mu.Unlock()
	if stopping {
		d.record(j, reasonShutdown, 0, 0, nil)
		d.outstanding.Done()
		return
	}
	defer d.requeueing.Done()
	select {
	case d.retries <- j:
	case <-d.ctx.Done():
		d.record(j, reasonShutdown, 0, 0, nil)
		d.outstanding.Done()
	}
}

// parseRetryAfter reads Retry-After in either form RFC 9110 §10.2.3 defines: delay-seconds or an
// HTTP date. An unparsable value, or a date already past, is treated as absent.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(max(secs, 0)) * time.Second
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(time.Until(at), 0)
	}
	return 0
}

// workCtx carries the terminating request's trace id on the dispatcher's own context, so shutdown
// cancels the work and the end of the request does not.
func (d *dispatcher) workCtx(traceID string) context.Context {
	return syscontext.WithTraceID(d.ctx, traceID)
}

// recordCtx carries the trace id on a context that shutdown does not cancel, so outcomes recorded
// during the drain still reach the logger and the publisher.
func (d *dispatcher) recordCtx(traceID string) context.Context {
	return syscontext.WithTraceID(context.Background(), traceID)
}

// recordDropped records a whole termination that will not be delivered: one event per participant,
// which the event stream needs, but a single warning for the termination, since a full queue means
// the system is overloaded and this runs on the terminating request.
func (d *dispatcher) recordDropped(t termination, reason failureReason) {
	ctx := d.recordCtx(t.traceID)
	d.logger.Warn(ctx, "Back-channel logout notifications dropped", log.String("sessionID", t.sessionID),
		log.Int("participants", len(t.appIDs)), log.String("reason", string(reason)))
	for _, appID := range t.appIDs {
		j := &job{traceID: t.traceID, sessionID: t.sessionID, subjectID: t.subjectID, appID: appID}
		d.publish(ctx, event.EventTypeBackchannelLogoutFailed, providers.StatusFailure, j, reason, 0, 0)
	}
}

// recordDelivered emits the success event for one job; the event is the record, so the log line
// is debug only.
func (d *dispatcher) recordDelivered(j *job, status int, elapsed time.Duration) {
	ctx := d.recordCtx(j.traceID)
	if d.logger.IsDebugEnabled() {
		d.logger.Debug(ctx, "Back-channel logout delivered", d.fields(j, status, elapsed)...)
	}
	d.publish(ctx, event.EventTypeBackchannelLogoutDelivered, providers.StatusSuccess, j, "", status, elapsed)
}

// record emits the failure event and the one warning logged for a job that will not be delivered.
func (d *dispatcher) record(j *job, reason failureReason, status int, elapsed time.Duration, err error) {
	ctx := d.recordCtx(j.traceID)
	d.logger.Warn(ctx, "Back-channel logout delivery failed", d.failureFields(j, reason, status, elapsed, err)...)
	d.publish(ctx, event.EventTypeBackchannelLogoutFailed, providers.StatusFailure, j, reason, status, elapsed)
}

// failureFields adds the reason, and the cause when there is one, to the recorded attributes.
func (d *dispatcher) failureFields(j *job, reason failureReason, status int, elapsed time.Duration,
	err error) []log.Field {
	fields := append(d.fields(j, status, elapsed), log.String("reason", string(reason)))
	if err != nil {
		fields = append(fields, log.Error(err))
	}
	return fields
}

// fields are the recorded attributes: never the token, the URI, bodies, or the subject.
func (d *dispatcher) fields(j *job, status int, elapsed time.Duration) []log.Field {
	return []log.Field{
		log.String("clientID", clientIDOf(j)),
		log.String("appID", j.appID),
		log.String("sessionID", j.sessionID),
		log.Int("attempt", j.attempt),
		log.Int("statusCode", status),
		log.Int("durationMs", int(elapsed.Milliseconds())),
	}
}

func (d *dispatcher) publish(ctx context.Context, eventType providers.EventType, status string, j *job,
	reason failureReason, httpStatus int, elapsed time.Duration) {
	if d.events == nil || !d.events.IsEnabled() {
		return
	}
	evt := event.NewEvent(j.traceID, string(eventType), event.ComponentBackchannelLogout).
		WithStatus(status).
		WithData(event.DataKey.ClientID, clientIDOf(j)).
		WithData(event.DataKey.EntityID, j.appID).
		WithData(event.DataKey.SessionID, j.sessionID).
		WithData(event.DataKey.AttemptNumber, j.attempt).
		WithData(event.DataKey.HTTPStatus, httpStatus).
		WithData(event.DataKey.DurationMs, elapsed.Milliseconds())
	if reason != "" {
		evt = evt.WithData(event.DataKey.Error, string(reason))
	}
	d.events.PublishEvent(ctx, evt)
}

func clientIDOf(j *job) string {
	if j.client == nil {
		return ""
	}
	return j.client.ClientID
}
