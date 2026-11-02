// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// applyGateway stands in for a data plane a version is applied to: it answers the store reads an apply
// makes as holding every value asked for, and takes each import slowly enough for another apply to
// arrive while one runs.
type applyGateway struct {
	server *httptest.Server
	delay  time.Duration
	// refuse makes every import fail, as a data plane that cannot take the version would.
	refuse  bool
	began   chan struct{}
	mu      sync.Mutex
	imports int
}

func newApplyGateway(delay time.Duration) *applyGateway {
	return startApplyGateway(delay, false)
}

// newRefusingApplyGateway stands in for a data plane that answers every import with a failure.
func newRefusingApplyGateway() *applyGateway {
	return startApplyGateway(0, true)
}

func startApplyGateway(delay time.Duration, refuse bool) *applyGateway {
	g := &applyGateway{delay: delay, refuse: refuse, began: make(chan struct{}, 8)}
	g.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("API-Key") != captureGatewayKey {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/variables" || r.URL.Path == "/secrets"):
			items := make([]map[string]string, 0)
			for _, name := range strings.Split(r.URL.Query().Get("names"), ",") {
				if name != "" {
					items = append(items, map[string]string{"name": name})
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"totalResults": len(items), "count": len(items),
				strings.TrimPrefix(r.URL.Path, "/"): items})
		case r.Method == http.MethodPost && r.URL.Path == "/import":
			g.mu.Lock()
			g.imports++
			g.mu.Unlock()
			if g.refuse {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			g.began <- struct{}{}
			time.Sleep(g.delay)
			_, _ = w.Write([]byte(`{"summary":{"totalDocuments":0,"imported":0,"failed":0},"results":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return g
}

// importsSoon waits for the gateway to have received at least n imports.
func (g *applyGateway) importsSoon(n int) bool {
	deadline := time.Now().Add(20 * time.Second)
	for {
		g.mu.Lock()
		count := g.imports
		g.mu.Unlock()
		if count >= n || time.Now().After(deadline) {
			return count >= n
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// registerGateway registers a stand-in as one of this control plane's gateways for one test.
func (ts *ControlPlaneTestSuite) registerGateway(server *httptest.Server) string {
	ts.T().Helper()
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	var registered struct {
		ID string `json:"id"`
	}
	status, raw := ts.call(http.MethodPost, "/gateways", map[string]any{
		"name":          unique("apply-gateway"),
		"baseUrl":       server.URL,
		"key":           captureGatewayKey,
		"caCertificate": string(certificate),
	}, &registered)
	ts.Require().Equal(http.StatusCreated, status, string(raw))
	ts.T().Cleanup(func() { ts.call(http.MethodDelete, "/gateways/"+registered.ID, nil, nil) })
	return registered.ID
}

// applyOutcome is what an apply answered, read outside the test's goroutine.
type applyOutcome struct {
	status        int
	raw           string
	Recorded      bool   `json:"recorded"`
	Queued        bool   `json:"queued"`
	QueuedVersion string `json:"queuedVersion"`
}

// applyLatest sends an apply of the latest version and reads its answer, without failing the test
// from another goroutine.
func applyLatest(gatewayID string) applyOutcome {
	outcome := applyOutcome{}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/gateways/"+gatewayID+"/apply",
		bytes.NewReader([]byte(`{}`)))
	if err != nil {
		outcome.raw = err.Error()
		return outcome
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		outcome.raw = err.Error()
		return outcome
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	outcome.status, outcome.raw = resp.StatusCode, string(body)
	_ = json.Unmarshal(body, &outcome)
	return outcome
}

// An apply of the latest version that arrives while another apply runs is queued rather than refused
// or run alongside, and is sent once the running one finishes, for the user who asked for it.
func (ts *ControlPlaneTestSuite) TestAnApplyOfTheLatestVersionIsQueuedBehindARunningOne() {
	g := newApplyGateway(2 * time.Second)
	ts.T().Cleanup(g.server.Close)
	gatewayID := ts.registerGateway(g.server)
	unit, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: unique("queued-apply"), Name: "Queued apply", Parent: &ts.ouID,
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(unit) }()
	ts.waitForCurrent(unit)

	first := make(chan applyOutcome, 1)
	go func() { first <- applyLatest(gatewayID) }()
	select {
	case <-g.began:
	case <-time.After(20 * time.Second):
		ts.FailNow("the first apply never reached the gateway")
	}

	queued := applyLatest(gatewayID)
	ts.Require().Equal(http.StatusAccepted, queued.status, queued.raw)
	ts.True(queued.Queued, "an apply arriving while another runs was not queued: %s", queued.raw)
	ts.NotEmpty(queued.QueuedVersion, "the queued apply does not say which version it will send")
	ts.False(queued.Recorded)

	select {
	case running := <-first:
		ts.Equal(http.StatusOK, running.status, running.raw)
		ts.True(running.Recorded, "the running apply was not recorded: %s", running.raw)
	case <-time.After(30 * time.Second):
		ts.FailNow("the running apply never answered")
	}
	ts.True(g.importsSoon(2), "the queued apply was not sent once the running one finished")

	type actor struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	var held struct {
		AppliedBy *actor `json:"appliedBy"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		status, raw := ts.call(http.MethodGet, "/gateways/"+gatewayID+"/applied-version", nil, &held)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		if held.AppliedBy != nil || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if ts.NotNil(held.AppliedBy, "the queued apply did not record who asked for it") {
		ts.Equal(testutils.TrustedIssuerAdminSubject, held.AppliedBy.ID)
		ts.Equal(testutils.TrustedIssuerAdminName, held.AppliedBy.Name)
	}
}
