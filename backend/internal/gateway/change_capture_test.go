// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// changeCaptureFor returns a ChangeCapture that counts its captures.
func changeCaptureFor(t *testing.T, capture func(ctx context.Context) error) (*ChangeCapture, *atomic.Int32) {
	t.Helper()
	count := &atomic.Int32{}
	c := NewChangeCapture()
	c.delay = 10 * time.Millisecond
	c.capture = func(ctx context.Context) error {
		count.Add(1)
		return capture(ctx)
	}
	return c, count
}

// waitForCapture waits for the capture a change scheduled to have run and nothing to be scheduled after it.
func waitForCapture(t *testing.T, c *ChangeCapture) {
	t.Helper()
	require.Eventually(t, func() bool { return !scheduled(c) }, 2*time.Second, 5*time.Millisecond, "no capture ran")
}

// waitForCaptures waits for at least n captures to have run.
func waitForCaptures(t *testing.T, count *atomic.Int32, n int32) {
	t.Helper()
	require.Eventually(t, func() bool { return count.Load() >= n }, 2*time.Second, 5*time.Millisecond,
		"only %d of %d captures ran", count.Load(), n)
}

// scheduled reports whether a capture of any deployment is waiting to run or running.
func scheduled(c *ChangeCapture) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, d := range c.deployments {
		if d.timer != nil || d.running {
			return true
		}
	}
	return false
}

// failuresOf reads how many captures of the deployment failed in a row.
func failuresOf(c *ChangeCapture, deploymentID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d := c.deployments[deploymentID]; d != nil {
		return d.failures
	}
	return 0
}

func answering(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) })
}

// A successful write to a configuration API is captured once the configuration settles; a read, a
// refused write, and a write elsewhere are not.
func TestAChangeToTheConfigurationIsCaptured(t *testing.T) {
	c, count := changeCaptureFor(t, func(context.Context) error { return nil })

	cases := []struct {
		method, path string
		status       int
		captured     bool
	}{
		{http.MethodPost, "/applications", http.StatusCreated, true},
		{http.MethodPut, "/organization-units/ou-1", http.StatusOK, true},
		{http.MethodDelete, "/design/themes/t-1", http.StatusNoContent, true},
		{http.MethodGet, "/applications", http.StatusOK, false},
		{http.MethodPost, "/applications", http.StatusBadRequest, false},
		{http.MethodPost, "/gateways", http.StatusCreated, false},
		{http.MethodPost, "/configuration-versions", http.StatusCreated, false},
		{http.MethodPost, "/oauth2/token", http.StatusOK, false},
		{http.MethodPost, "/applicationsx", http.StatusCreated, false},
	}
	for _, tc := range cases {
		before := count.Load()
		recorder := httptest.NewRecorder()
		c.Middleware(answering(tc.status)).ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		assert.Equal(t, tc.status, recorder.Code, "%s %s", tc.method, tc.path)
		if tc.captured {
			waitForCaptures(t, count, before+1)
			// The capture counts before it finishes; the next case looks at what is scheduled.
			waitForCapture(t, c)
			assert.Equal(t, before+1, count.Load(), "%s %s was not captured", tc.method, tc.path)
		} else {
			// A capture is scheduled within the request, so none is scheduled once it has answered.
			assert.False(t, scheduled(c), "%s %s scheduled a capture", tc.method, tc.path)
			assert.Equal(t, before, count.Load(), "%s %s was captured", tc.method, tc.path)
		}
	}
}

// A capture after a change is kept for the user who made the last change before it, though it runs
// as the system's own work.
func TestACaptureAfterAChangeIsKeptForWhoMadeIt(t *testing.T) {
	captured := make(chan *Actor, 1)
	c, _ := changeCaptureFor(t, func(ctx context.Context) error {
		captured <- actorOf(ctx)
		return nil
	})

	request := httptest.NewRequest(http.MethodPost, "/applications", nil).WithContext(
		signedIn("u-1", map[string]interface{}{"username": "alice"}))
	c.Middleware(answering(http.StatusCreated)).ServeHTTP(httptest.NewRecorder(), request)
	waitForCapture(t, c)

	assert.Equal(t, &Actor{ID: "u-1"}, <-captured)
}

// A change whose request names no user is captured for no one, not for whoever changed something
// before it; a change that arrives while a capture runs keeps its user for the capture after.
func TestAChangeNamingNoUserIsCapturedForNoOne(t *testing.T) {
	captured := make(chan *Actor, 2)
	c, _ := changeCaptureFor(t, func(ctx context.Context) error {
		captured <- actorOf(ctx)
		return nil
	})
	c.delay = time.Hour

	change := func(ctx context.Context) {
		request := httptest.NewRequest(http.MethodPost, "/applications", nil).WithContext(ctx)
		c.Middleware(answering(http.StatusCreated)).ServeHTTP(httptest.NewRecorder(), request)
	}
	change(signedIn("u-1", nil))
	c.Flush(context.Background())
	assert.Equal(t, &Actor{ID: "u-1"}, <-captured)

	change(context.Background())
	c.Flush(context.Background())
	assert.Nil(t, <-captured, "a change naming no user was captured for the user before it")
}

// Changes close together are captured once, and a change while a capture runs is captured after it.
func TestChangesAreCapturedOnceTheySettle(t *testing.T) {
	release := make(chan struct{})
	first := true
	c, count := changeCaptureFor(t, func(context.Context) error {
		if first {
			first = false
			<-release
		}
		return nil
	})

	c.Notify(context.Background())
	c.Notify(context.Background())
	c.Notify(context.Background())
	// The first capture holds until released, so once one has started no other can.
	require.Eventually(t, func() bool { return count.Load() == 1 }, 2*time.Second, 5*time.Millisecond)
	assert.Equal(t, int32(1), count.Load(), "changes close together were captured apart")

	c.Notify(context.Background()) // while the first capture runs
	close(release)
	waitForCaptures(t, count, 2)
	waitForCapture(t, c)
	assert.Equal(t, int32(2), count.Load(), "the change made during a capture was not captured")
}

// A capture that fails is logged and does not stop later changes being captured; without a binding,
// or without a ChangeCapture at all, nothing happens.
func TestAFailedCaptureDoesNotStopTheNext(t *testing.T) {
	failing := atomic.Bool{}
	failing.Store(true)
	c, count := changeCaptureFor(t, func(context.Context) error {
		if failing.Load() {
			return errors.New("export failed")
		}
		return nil
	})
	c.Notify(context.Background())
	waitForCaptures(t, count, 1)
	failing.Store(false)
	c.Notify(context.Background())
	waitForCaptures(t, count, 2)
	waitForCapture(t, c)

	unbound := NewChangeCapture()
	unbound.Notify(context.Background())

	var none *ChangeCapture
	handler := answering(http.StatusCreated)
	require.NotNil(t, none.Middleware(handler))
	none.Notify(context.Background())
}

// A bound ChangeCapture captures the configuration as the current version.
func TestABoundChangeCaptureKeepsTheCurrentVersion(t *testing.T) {
	f := newVersionFixture(t)
	c := NewChangeCapture()
	c.delay = time.Millisecond
	c.bind(f.svc.(*versionService))
	f.exporter.response = exportOf("", "ou-a")

	c.Notify(context.Background())
	waitForCapture(t, c)

	require.Len(t, f.versions.versions, 1)
	assert.True(t, f.versions.versions[0].Current)
	// The answer is discarded, so the user it names is not looked up at the issuer.
	assert.Zero(t, f.names.fills, "the capture after a change looked a name up")

	f.exporter.err = &tidcommon.InternalServerError
	require.Error(t, c.capture(context.Background()))
}

// A capture that fails is tried again on its own, each time after a longer wait, until one succeeds;
// then nothing more is scheduled. A change that arrives meanwhile is captured as usual.
func TestAFailedCaptureIsTriedAgainUntilItSucceeds(t *testing.T) {
	remaining := atomic.Int32{}
	remaining.Store(2)
	c, count := changeCaptureFor(t, func(context.Context) error {
		if remaining.Add(-1) >= 0 {
			return errors.New("export failed")
		}
		return nil
	})

	c.Notify(context.Background())
	waitForCaptures(t, count, 3)

	assert.Equal(t, int32(3), count.Load())
	assert.Eventually(t, func() bool { return !scheduled(c) }, time.Second, 5*time.Millisecond,
		"a capture was scheduled after one succeeded")
	assert.Equal(t, 0, failuresOf(c, deployment.Resolve(context.Background())))

	assert.Equal(t, c.delay, c.retryDelay(1))
	assert.Equal(t, 4*c.delay, c.retryDelay(3))
	assert.Equal(t, retryDelayMax, c.retryDelay(40))
}

// Each deployment is captured on its own: a change to one captures that one, with the request's
// deployment carried into the capture, and leaves the other's capture where it was.
func TestEachDeploymentIsCapturedOnItsOwn(t *testing.T) {
	var mu sync.Mutex
	captured := map[string]int{}
	c, count := changeCaptureFor(t, func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		captured[deployment.Resolve(ctx)]++
		return nil
	})
	c.delay = time.Hour
	change := func(deploymentID string) {
		request := httptest.NewRequest(http.MethodPost, "/applications", nil).
			WithContext(deployment.WithID(context.Background(), deploymentID))
		c.Middleware(answering(http.StatusCreated)).ServeHTTP(httptest.NewRecorder(), request)
	}

	change("acme")
	change("globex")
	change("acme")
	c.Flush(context.Background())
	assert.Equal(t, int32(2), count.Load())

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, map[string]int{"acme": 1, "globex": 1}, captured)
}

// A timer that fires just as a change replaces it does not capture: the capture the change scheduled
// answers for both, so a burst never captures the same deployment twice at once.
func TestATimerReplacedAsItFiredDoesNotCapture(t *testing.T) {
	c, count := changeCaptureFor(t, func(context.Context) error { return nil })
	c.delay = time.Hour
	c.Notify(context.Background())
	c.Notify(context.Background())

	c.mu.Lock()
	d := c.deployments[deployment.Resolve(context.Background())]
	replaced, current := d.generation-1, d.generation
	c.mu.Unlock()
	c.run(deployment.Resolve(context.Background()), replaced)
	assert.Equal(t, int32(0), count.Load(), "the replaced timer captured")
	assert.True(t, scheduled(c), "the replaced timer unscheduled the capture")

	c.run(deployment.Resolve(context.Background()), current)
	assert.Equal(t, int32(1), count.Load())
}

// A change still waiting for its capture when the server stops is captured by the flush, within the
// shutdown's bound, and one the flush cannot finish in time is left for the next start.
func TestAFlushCapturesWhatIsStillWaiting(t *testing.T) {
	release := make(chan struct{})
	c, count := changeCaptureFor(t, func(context.Context) error {
		<-release
		return nil
	})
	c.delay = time.Hour
	c.Notify(context.Background())
	require.True(t, scheduled(c))

	flushed := make(chan struct{})
	go func() {
		c.Flush(context.Background())
		close(flushed)
	}()
	require.Eventually(t, func() bool { return count.Load() == 1 }, time.Second, time.Millisecond,
		"the flush did not run the waiting capture")
	select {
	case <-flushed:
		t.Fatal("the flush returned before the capture finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	<-flushed
	assert.False(t, scheduled(c))

	// A capture the shutdown's bound does not allow for is given up on, not waited for.
	stuck := make(chan struct{})
	c2, _ := changeCaptureFor(t, func(context.Context) error { <-stuck; return nil })
	c2.delay = time.Hour
	c2.Notify(context.Background())
	bounded, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c2.Flush(bounded)
	close(stuck)
	NewChangeCapture().Flush(context.Background())
}
