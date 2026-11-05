// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package push announces recorded revocations to remote Resource Servers. Its dispatcher listens for
// deny-list writes and POSTs each one to every configured receiver as a signed Security Event Token
// (RFC 8417) over push delivery (RFC 8935), in process, asynchronously, with bounded retry.
package push

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	sharedrevocation "github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// claimAudience is the audience claim (RFC 7519 §4.1.3) naming the receiver an event token is for.
const claimAudience = "aud"

// DispatcherInterface turns recorded revocations into event token deliveries. OnEvent copies the
// event onto a bounded queue for every receiver and returns; workers build a token per attempt, POST
// it, and retry transient failures.
type DispatcherInterface interface {
	eventlistener.Listener[sharedrevocation.Event]
	// Start launches the workers. It is not called on the bootstrap path.
	Start(ctx context.Context)
	// Stop refuses new events, lets queued and in-flight deliveries finish within the shutdown
	// budget, and drops whatever is left. It is safe to call more than once.
	Stop()
}

// dispatcher is the DispatcherInterface implementation: a bounded queue, a worker pool and a retry
// scheduler, all in process and not persisted.
type dispatcher struct {
	cfg engineconfig.TokenRevocationPushConfig
	// retryDelay is cfg's whole seconds as a duration, so tests can shorten it.
	retryDelay time.Duration

	jwtService jwt.JWTServiceInterface
	http       syshttp.HTTPClientInterface
	logger     *log.Logger

	queue   chan *job
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup

	mu       sync.Mutex
	started  bool
	stopping bool
	// pending holds the retries waiting for their timer.
	pending map[*job]*time.Timer
}

// newDispatcher creates a stopped dispatcher from the deployment configuration, which is validated
// at startup, so every count and duration is positive.
func newDispatcher(cfg engineconfig.TokenRevocationPushConfig, jwtService jwt.JWTServiceInterface,
	httpClient syshttp.HTTPClientInterface) *dispatcher {
	return &dispatcher{
		cfg:        cfg,
		retryDelay: time.Duration(cfg.RetryDelay) * time.Second,
		jwtService: jwtService,
		http:       httpClient,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RevocationPushDispatcher")),
		queue:      make(chan *job, cfg.QueueSize),
		pending:    make(map[*job]*time.Timer),
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
	for range workerCount {
		d.workers.Add(1)
		go d.work()
	}
}

// OnEvent implements eventlistener.Listener. It never blocks.
func (d *dispatcher) OnEvent(ctx context.Context, event sharedrevocation.Event) {
	traceID := syscontext.GetTraceID(ctx)
	for _, receiver := range d.cfg.Receivers {
		d.enqueue(&job{traceID: traceID, event: event, receiver: receiver, attempt: 1})
	}
}

// Stop implements DispatcherInterface. A Stop before Start does nothing.
func (d *dispatcher) Stop() {
	d.mu.Lock()
	if !d.started || d.stopping {
		d.mu.Unlock()
		return
	}
	d.stopping = true
	var parked []*job
	for j, timer := range d.pending {
		if timer.Stop() {
			delete(d.pending, j)
			parked = append(parked, j)
		}
	}
	// Every sender holds the lock while it sends, so none is mid-send once this closes the queue.
	close(d.queue)
	d.mu.Unlock()
	for _, j := range parked {
		d.drop(j, reasonShutdown)
	}

	done := make(chan struct{})
	go func() {
		d.workers.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownBudget):
		d.cancel()
		<-done
	}
	d.cancel()
}

// enqueue hands a job to the workers without blocking, dropping it when the dispatcher is not
// running or its queue is full.
func (d *dispatcher) enqueue(j *job) {
	d.mu.Lock()
	reason := failureReason("")
	switch {
	case !d.started:
		reason = reasonNotStarted
	case d.stopping:
		reason = reasonShutdown
	default:
		select {
		case d.queue <- j:
		default:
			reason = reasonQueueFull
		}
	}
	d.mu.Unlock()
	if reason != "" {
		d.drop(j, reason)
	}
}

// work is one worker: it delivers queued jobs until the queue is closed, dropping the rest once
// shutdown has been forced.
func (d *dispatcher) work() {
	defer d.workers.Done()
	for j := range d.queue {
		if d.ctx.Err() != nil {
			d.drop(j, reasonShutdown)
			continue
		}
		d.attempt(j)
	}
}

// attempt builds a fresh token, POSTs it, and settles the job or schedules the next attempt.
func (d *dispatcher) attempt(j *job) {
	ctx := syscontext.WithTraceID(d.ctx, j.traceID)
	token, err := d.buildToken(ctx, j)
	if err != nil {
		d.settle(j, outcome{retryable: true, reason: reasonServerError, err: err})
		return
	}
	d.settle(j, d.post(ctx, j.receiver.URL, token))
}

// buildToken signs the Security Event Token announcing the job's event to its receiver.
func (d *dispatcher) buildToken(ctx context.Context, j *job) (string, error) {
	claims := j.event.Claims()
	claims[claimAudience] = j.receiver.Audience
	token, _, svcErr := d.jwtService.GenerateJWT(ctx, "", "", eventValidityPeriod, claims,
		sharedrevocation.SecurityEventTokenType, "")
	if svcErr != nil {
		return "", fmt.Errorf("failed to build event token: %s", svcErr.Code)
	}
	return token, nil
}

// post performs one delivery request and classifies the response (RFC 8935 §2.3).
func (d *dispatcher) post(ctx context.Context, receiverURL, token string) outcome {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, receiverURL, strings.NewReader(token))
	if err != nil {
		return outcome{reason: reasonRejected, err: err}
	}
	req.Header.Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeSecurityEventToken)
	req.Header.Set(serverconst.AcceptHeaderName, serverconst.ContentTypeJSON)

	resp, err := d.http.Do(req)
	if err != nil {
		// A url.Error repeats the request URL; keep only the cause it wraps.
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
	case resp.StatusCode == http.StatusAccepted:
		return outcome{delivered: true, status: resp.StatusCode}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return outcome{retryable: true, reason: reasonServerError, status: resp.StatusCode}
	default:
		return outcome{reason: reasonRejected, status: resp.StatusCode}
	}
}

// settle logs a terminal outcome or schedules the next attempt.
func (d *dispatcher) settle(j *job, o outcome) {
	if o.delivered {
		if d.logger.IsDebugEnabled() {
			d.logger.Debug(d.recordCtx(j), "Revocation event delivered", d.fields(j, o.status)...)
		}
		return
	}
	if !o.retryable || j.attempt >= d.cfg.MaxAttempts {
		d.logger.Warn(d.recordCtx(j), "Revocation event delivery failed",
			d.failureFields(j, o.reason, o.status, o.err)...)
		return
	}
	j.attempt++
	d.park(j, d.retryDelay<<(j.attempt-2))
}

// park holds the job on a timer, not a worker, until its next attempt is due, or drops it once Stop
// has begun.
func (d *dispatcher) park(j *job, delay time.Duration) {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		d.drop(j, reasonShutdown)
		return
	}
	d.pending[j] = time.AfterFunc(delay, func() { d.retry(j) })
	d.mu.Unlock()
}

// retry hands a job whose wait is over back to the workers.
func (d *dispatcher) retry(j *job) {
	d.mu.Lock()
	delete(d.pending, j)
	d.mu.Unlock()
	d.enqueue(j)
}

// drop logs a job that will not be delivered.
func (d *dispatcher) drop(j *job, reason failureReason) {
	d.logger.Warn(d.recordCtx(j), "Revocation event dropped", d.failureFields(j, reason, 0, nil)...)
}

// recordCtx carries the trace id on a context that shutdown does not cancel, so outcomes recorded
// during the drain still reach the logger.
func (d *dispatcher) recordCtx(j *job) context.Context {
	return syscontext.WithTraceID(context.Background(), j.traceID)
}

// failureFields adds the reason, and the cause when there is one, to the logged attributes.
func (d *dispatcher) failureFields(j *job, reason failureReason, status int, err error) []log.Field {
	fields := append(d.fields(j, status), log.String("reason", string(reason)))
	if err != nil {
		fields = append(fields, log.Error(err))
	}
	return fields
}

// fields are the logged attributes: never the token, the receiver's URL, or the revoked values.
func (d *dispatcher) fields(j *job, status int) []log.Field {
	return []log.Field{
		log.String("eventType", j.event.EventType()),
		log.String("receiverAudience", j.receiver.Audience),
		log.Int("attempt", j.attempt),
		log.Int("statusCode", status),
	}
}
