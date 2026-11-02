// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// changeDelay is how long a capture waits after a change for more to follow, so a burst of writes,
// such as an import or a wizard creating several resources, is captured once.
const changeDelay = 2 * time.Second

// changeCaptureTimeout bounds one capture, which exports the whole configuration.
const changeCaptureTimeout = time.Minute

// retryDelayMax bounds how long a capture that failed waits before it is tried again.
const retryDelayMax = time.Minute

// configurationPaths are the APIs whose writes change the configuration a version captures. Writes
// elsewhere, such as signing in, the gateways and their values, or the versions themselves, do not.
// The list is the set of resource types the export covers, so a resource type that joins the export
// is added here with it.
var configurationPaths = []string{
	"/agent-types", "/agents", "/applications", "/authorization-detail-types", "/connections", "/design",
	"/flows", "/groups", "/i18n", "/import", "/notification-templates", "/openid4vci", "/openid4vp",
	"/organization-units", "/ou", "/resource-servers", "/roles", "/scim", "/server-config", "/user-types",
	"/users",
}

// ChangeCapture keeps the current version up to date as the configuration changes: a successful
// write to a configuration API schedules a capture of the configuration as current, which the
// versions keep in place until a gateway holds it. Nothing is captured while nothing changes.
//
// Each deployment is captured on its own: a change to one schedules a capture of that one, and
// neither waits on, nor resets, another's. A capture is kept for the user who made the last change
// before it.
type ChangeCapture struct {
	capture func(ctx context.Context) error
	delay   time.Duration
	logger  *log.Logger

	mu          sync.Mutex
	deployments map[string]*deploymentCapture
}

// deploymentCapture is the capture one deployment has scheduled or running.
type deploymentCapture struct {
	timer *time.Timer
	// generation numbers the timer last armed. A timer that fired just as another replaced it finds
	// the generation moved on and does nothing, so the one that replaced it captures alone.
	generation uint64
	running    bool
	pending    bool
	actor      *Actor
	// failures counts the captures that failed in a row; the next is tried again after a delay that
	// grows with it, so a failing exporter is not asked every two seconds, and a passing one soon after.
	failures int
}

// NewChangeCapture returns a ChangeCapture that does nothing until Initialize binds it to the versions.
func NewChangeCapture() *ChangeCapture {
	return &ChangeCapture{
		delay:       changeDelay,
		logger:      log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayChangeCapture")),
		deployments: map[string]*deploymentCapture{},
	}
}

func (c *ChangeCapture) bind(versions *versionService) {
	c.capture = func(ctx context.Context) error {
		if _, svcErr := versions.capture(ctx, CaptureRequest{Current: true}); svcErr != nil {
			return fmt.Errorf("%s: %s", svcErr.Code, svcErr.Error.DefaultValue)
		}
		versions.applyToDefaultGateway(ctx)
		return nil
	}
}

// Middleware notes each successful write to a configuration API and schedules a capture for it.
func (c *ChangeCapture) Middleware(next http.Handler) http.Handler {
	if c == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !changesConfiguration(r) {
			next.ServeHTTP(w, r)
			return
		}
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.status >= 200 && recorder.status < 300 {
			c.Notify(r.Context())
		}
	})
}

// changesConfiguration reports whether a request writes to a configuration API.
func changesConfiguration(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	for _, prefix := range configurationPaths {
		if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
			return true
		}
	}
	return false
}

// Notify schedules a capture of the deployment ctx acts for once its configuration has stopped
// changing for a moment, kept for the user ctx acts for. A change while a capture runs schedules one
// more after it, so the last change is always captured.
func (c *ChangeCapture) Notify(ctx context.Context) {
	if c == nil || c.capture == nil {
		return
	}
	c.notify(deployment.Resolve(ctx), actorOf(ctx), false)
}

// notify schedules a capture for a change actor made, nil when the change names no user, so the
// capture is for no one rather than for whoever changed something last. With keep set, the capture
// stays for the user it already had: the re-run after a change that arrived while a capture ran.
func (c *ChangeCapture) notify(deploymentID string, actor *Actor, keep bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d := c.deployments[deploymentID]
	if d == nil {
		d = &deploymentCapture{}
		c.deployments[deploymentID] = d
	}
	if !keep {
		d.actor = actor
	}
	if d.running {
		d.pending = true
		return
	}
	if d.timer != nil {
		d.timer.Stop()
	}
	c.arm(d, deploymentID, c.delay)
}

// arm schedules the deployment's capture after delay. Called with c.mu held.
func (c *ChangeCapture) arm(d *deploymentCapture, deploymentID string, delay time.Duration) {
	d.generation++
	generation := d.generation
	d.timer = time.AfterFunc(delay, func() { c.run(deploymentID, generation) })
}

// run captures the deployment for the timer of the given generation, unless a later one has been
// armed since, whose capture answers for this one too.
func (c *ChangeCapture) run(deploymentID string, generation uint64) {
	c.mu.Lock()
	d := c.deployments[deploymentID]
	if d.generation != generation {
		c.mu.Unlock()
		return
	}
	d.timer = nil
	d.running = true
	actor := d.actor
	c.mu.Unlock()

	// The capture is the system's own work, not the request's that set it off: it exports the whole
	// configuration whoever made the change, so a version never depends on what one caller may read.
	// It is still the deployment's that changed, and kept for the user whose change it follows.
	ctx, cancel := context.WithTimeout(withActor(deployment.WithID(security.WithRuntimeContext(
		context.Background()), deploymentID), actor), changeCaptureTimeout)
	err := c.capture(ctx)
	if err != nil {
		c.logger.Warn(ctx, "Failed to capture the configuration after a change; it will be tried again",
			log.Error(err))
	}
	cancel()

	c.mu.Lock()
	d.running = false
	again := d.pending
	d.pending = false
	if err != nil {
		d.failures++
	} else {
		d.failures = 0
	}
	// A capture that failed is tried again on its own, so the current version does not stay stale
	// until something else changes. A change that arrived meanwhile schedules the next capture anyway.
	if err != nil && !again && d.timer == nil {
		c.arm(d, deploymentID, c.retryDelay(d.failures))
	}
	c.mu.Unlock()
	if again {
		c.notify(deploymentID, nil, true)
	}
}

// Flush captures every change still waiting for its moment, and waits for a capture that runs,
// so a change made just before the server stops is not lost. It gives up when ctx ends, as a
// shutdown's bound does, and a capture it could not finish is left for the next start's first
// change.
func (c *ChangeCapture) Flush(ctx context.Context) {
	if c == nil || c.capture == nil {
		return
	}
	type dueCapture struct {
		deploymentID string
		generation   uint64
	}
	c.mu.Lock()
	var due []dueCapture
	for id, d := range c.deployments {
		if d.timer != nil && d.timer.Stop() {
			d.timer = nil
			due = append(due, dueCapture{id, d.generation})
		}
	}
	c.mu.Unlock()

	done := make(chan struct{})
	go func() {
		for _, capture := range due {
			c.run(capture.deploymentID, capture.generation)
		}
		// A capture that was already running, or one a change set off meanwhile, finishes too.
		for !c.idle() {
			time.Sleep(10 * time.Millisecond)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		c.logger.Warn(ctx, "Stopped before every change was captured; the next change will capture them")
	}
}

// idle reports whether no capture is scheduled or running for any deployment.
func (c *ChangeCapture) idle() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.deployments {
		if d.timer != nil || d.running {
			return false
		}
	}
	return true
}

// retryDelay is how long the next attempt waits after the given failures in a row: the usual delay,
// doubled for each failure after the first, up to retryDelayMax.
func (c *ChangeCapture) retryDelay(failures int) time.Duration {
	delay := c.delay
	for i := 1; i < failures && delay < retryDelayMax; i++ {
		delay *= 2
	}
	if delay > retryDelayMax {
		return retryDelayMax
	}
	return delay
}

// statusRecorder remembers the status a handler answered with.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(body)
}

// Unwrap returns the wrapped ResponseWriter, so http.ResponseController reaches the connection.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
