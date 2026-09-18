// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = "https://localhost:8095"

// gateway is the shape a read returns. It has no key field, which is the point: the management
// token a registration carries is never returned, so there is nothing here to decode it into.
type gateway struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DataPlaneID   string `json:"dataPlaneId"`
	BaseURL       string `json:"baseUrl"`
	CACertificate string `json:"caCertificate,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type errorResponse struct {
	Code string `json:"code"`
}

type GatewayAPITestSuite struct {
	suite.Suite
	registered []string
}

func TestGatewayAPITestSuite(t *testing.T) {
	suite.Run(t, new(GatewayAPITestSuite))
}

// TearDownTest removes whatever a test registered, so the deployment's gateway bound does not leak
// from one test into the next.
func (ts *GatewayAPITestSuite) TearDownTest() {
	for _, id := range ts.registered {
		ts.deleteGateway(id)
	}
	ts.registered = nil
}

func (ts *GatewayAPITestSuite) TestRegisterAndReadBack() {
	created := ts.register(map[string]string{
		"name":        "integration-primary",
		"dataPlaneId": "integration-dp-1",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusCreated)

	ts.Require().NotEmpty(created.ID)
	ts.Equal("integration-primary", created.Name)
	ts.Equal("integration-dp-1", created.DataPlaneID)
	ts.Equal("https://dp.integration.test:8090", created.BaseURL)
	// The database assigns these, so an empty value means the response was built rather than read.
	ts.NotEmpty(created.CreatedAt)
	ts.NotEmpty(created.UpdatedAt)

	fetched := ts.getGateway(created.ID, http.StatusOK)
	ts.Equal(created.ID, fetched.ID)
	ts.Equal("integration-dp-1", fetched.DataPlaneID)
}

// The management token is write-only. Whatever a read returns, it is not the credential.
func (ts *GatewayAPITestSuite) TestAReadNeverReturnsTheKey() {
	const key = "a-very-recognizable-management-token"
	created := ts.register(map[string]string{
		"name":        "integration-secret-check",
		"dataPlaneId": "integration-dp-secret",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         key,
	}, http.StatusCreated)

	for _, path := range []string{"/gateways/" + created.ID, "/gateways"} {
		body := ts.rawGet(path)
		ts.NotContains(body, key, "the management token was returned by GET %s", path)
		ts.NotContains(body, `"key"`, "GET %s returned a key field", path)
	}
}

// One data plane registers once, whatever name it is given the second time.
func (ts *GatewayAPITestSuite) TestADataPlaneRegistersOnce() {
	ts.register(map[string]string{
		"name":        "integration-first",
		"dataPlaneId": "integration-dp-unique",
		"baseUrl":     "https://internal.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusCreated)

	code := ts.registerExpectingError(map[string]string{
		"name":        "integration-second",
		"dataPlaneId": "integration-dp-unique",
		"baseUrl":     "https://external.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusConflict)
	ts.Equal("GTW-1008", code)
}

func (ts *GatewayAPITestSuite) TestANameRegistersOnce() {
	ts.register(map[string]string{
		"name":        "integration-duplicate-name",
		"dataPlaneId": "integration-dp-name-a",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusCreated)

	code := ts.registerExpectingError(map[string]string{
		"name":        "integration-duplicate-name",
		"dataPlaneId": "integration-dp-name-b",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusConflict)
	ts.Equal("GTW-1004", code)
}

// A registration that could never be reached is refused where the caller can still see why.
func (ts *GatewayAPITestSuite) TestAnUnusableRegistrationIsRefused() {
	for name, body := range map[string]map[string]string{
		"no data plane id": {"name": "x", "baseUrl": "https://dp.test", "key": "k"},
		"no key":           {"name": "x", "dataPlaneId": "d", "baseUrl": "https://dp.test"},
		"no base url":      {"name": "x", "dataPlaneId": "d", "key": "k"},
		"base url is a slash": {
			"name": "x", "dataPlaneId": "d", "baseUrl": "/", "key": "k"},
		"base url is a scheme": {
			"name": "x", "dataPlaneId": "d", "baseUrl": "https://", "key": "k"},
	} {
		ts.Run(name, func() {
			ts.registerExpectingError(body, http.StatusBadRequest)
		})
	}
}

func (ts *GatewayAPITestSuite) TestListReturnsWhatWasRegistered() {
	created := ts.register(map[string]string{
		"name":        "integration-listed",
		"dataPlaneId": "integration-dp-listed",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusCreated)

	req, err := http.NewRequest(http.MethodGet, testServerURL+"/gateways", nil)
	ts.Require().NoError(err)
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var listed []gateway
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listed))

	found := false
	for _, g := range listed {
		if g.ID == created.ID {
			found = true
			ts.Equal("integration-dp-listed", g.DataPlaneID)
		}
	}
	ts.True(found, "the registered gateway was not in the listing")
}

func (ts *GatewayAPITestSuite) TestDeleteRemovesIt() {
	created := ts.register(map[string]string{
		"name":        "integration-removable",
		"dataPlaneId": "integration-dp-removable",
		"baseUrl":     "https://dp.integration.test:8090",
		"key":         "integration-management-token",
	}, http.StatusCreated)

	ts.deleteGateway(created.ID)
	ts.registered = nil

	ts.getGateway(created.ID, http.StatusNotFound)
}

func (ts *GatewayAPITestSuite) TestReadingOneThatIsNotRegistered() {
	ts.getGateway("00000000-0000-0000-0000-000000000000", http.StatusNotFound)
}

// ---- helpers ----

func (ts *GatewayAPITestSuite) register(body map[string]string, wantStatus int) gateway {
	ts.T().Helper()

	resp := ts.post("/gateways", body)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(wantStatus, resp.StatusCode)

	var created gateway
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.registered = append(ts.registered, created.ID)
	return created
}

func (ts *GatewayAPITestSuite) registerExpectingError(body map[string]string, wantStatus int) string {
	ts.T().Helper()

	resp := ts.post("/gateways", body)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(wantStatus, resp.StatusCode)

	var failure errorResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&failure))
	return failure.Code
}

func (ts *GatewayAPITestSuite) post(path string, body map[string]string) *http.Response {
	ts.T().Helper()

	encoded, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+path, bytes.NewReader(encoded))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	return resp
}

func (ts *GatewayAPITestSuite) getGateway(id string, wantStatus int) gateway {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/gateways/%s", testServerURL, id), nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(wantStatus, resp.StatusCode)

	var fetched gateway
	if wantStatus == http.StatusOK {
		ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&fetched))
	}
	return fetched
}

func (ts *GatewayAPITestSuite) rawGet(path string) string {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+path, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return string(body)
}

func (ts *GatewayAPITestSuite) deleteGateway(id string) {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/gateways/%s", testServerURL, id), nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
}
