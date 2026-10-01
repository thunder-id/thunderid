// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// ImportGatewaySuite covers importing a gateway through /import.
//
// This is the one-time path, distinct from a gateway declared in config/resources/gateways: what it
// creates is a database row that outlives the document, where a declared gateway is held in memory
// and reconciled from its file on every start.
type ImportGatewaySuite struct {
	suite.Suite
	client *http.Client
	// created holds anything a test failed to remove itself, cleared in teardown. Each test removes
	// what it registered as it finishes: the configured bound counts every gateway the deployment
	// holds, so letting them accumulate across the suite exhausts it.
	created []string
}

func TestImportGatewaySuite(t *testing.T) {
	suite.Run(t, new(ImportGatewaySuite))
}

func (suite *ImportGatewaySuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()
}

func (suite *ImportGatewaySuite) TearDownSuite() {
	for _, id := range suite.created {
		suite.removeGateway(id)
	}
}

// removeGateway deletes a gateway, ignoring the answer: it is cleanup, and a test that already
// failed should report its own reason rather than a teardown error on top.
func (suite *ImportGatewaySuite) removeGateway(id string) {
	if id == "" {
		return
	}
	req, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+"/gateways/"+id, nil)
	if err != nil {
		return
	}
	if resp, doErr := suite.client.Do(req); doErr == nil {
		_ = resp.Body.Close()
	}
}

// An import registers a gateway, and what it registers is readable through the Gateway API
// afterwards: the two are the same records.
func (suite *ImportGatewaySuite) TestImportRegistersAGateway() {
	name := fmt.Sprintf("imported-gateway-%d", time.Now().UnixNano())
	baseURL := "https://" + name + ".test:8090"
	resp := suite.importGateway(name, baseURL, "the-imported-key")

	suite.Require().Equal(1, resp.Summary.Imported, "the gateway was not imported: %+v", resp.Results)
	suite.Require().Len(resp.Results, 1)

	result := resp.Results[0]
	suite.Equal("gateway", result.ResourceType)
	suite.Equal("success", result.Status)
	suite.Require().NotEmpty(result.ResourceID, "the import returned no gateway id")
	defer suite.removeGateway(result.ResourceID)

	// A real registration, not just a reported one.
	status, body := suite.get("/gateways/" + result.ResourceID)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var registered struct {
		Name    string `json:"name"`
		BaseURL string `json:"baseUrl"`
	}
	suite.Require().NoError(json.Unmarshal(body, &registered))
	suite.Equal(name, registered.Name)
	suite.Equal(baseURL, registered.BaseURL)

	// The key an import carried is no more readable than one the API issued.
	suite.NotContains(string(body), "the-imported-key", "the imported key was returned by a read")
}

// Re-importing the same document updates the gateway rather than adding a second one, which is what
// makes re-running bootstrap over an existing deployment safe.
func (suite *ImportGatewaySuite) TestReimportingUpdatesRatherThanDuplicating() {
	name := fmt.Sprintf("reimported-gateway-%d", time.Now().UnixNano())
	first := suite.importGateway(name, "https://"+name+".test:8090", "the-imported-key")
	suite.Require().Equal(1, first.Summary.Imported, "results: %+v", first.Results)
	id := first.Results[0].ResourceID
	suite.Require().NotEmpty(id)
	defer suite.removeGateway(id)

	// The second document names no key, which must leave the stored credential alone.
	moved := "https://" + name + ".moved.test:8090"
	second := suite.importGateway(name, moved, "")

	suite.Require().Equal(1, second.Summary.Imported, "results: %+v", second.Results)
	suite.Equal(id, second.Results[0].ResourceID, "re-importing made a second gateway")

	status, body := suite.get("/gateways/" + id)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var registered struct {
		BaseURL string `json:"baseUrl"`
	}
	suite.Require().NoError(json.Unmarshal(body, &registered))
	suite.Equal(moved, registered.BaseURL, "the update did not take")

	// And there is still exactly one gateway of that name.
	suite.Equal(1, suite.countGatewaysNamed(name), "re-importing left more than one gateway")
}

// A document missing what it takes to reach a gateway is reported as a failed item rather than
// registering something unusable.
func (suite *ImportGatewaySuite) TestImportRefusesAGatewayWithoutABaseURL() {
	name := fmt.Sprintf("incomplete-gateway-%d", time.Now().UnixNano())
	payload := suite.marshal(importRequest{
		Content: fmt.Sprintf("resource_type: gateway\nname: %s\n", name),
		Options: runtimeOptions(),
	})

	status, body := suite.post("/import", payload)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	suite.Equal(1, resp.Summary.Failed, "an incomplete gateway was imported: %+v", resp.Results)
	suite.Require().Len(resp.Results, 1)
	suite.Equal("failed", resp.Results[0].Status)
	suite.Equal(0, suite.countGatewaysNamed(name), "a refused document still registered a gateway")
}

// importGateway posts one gateway document and returns the decoded response.
func (suite *ImportGatewaySuite) importGateway(name, baseURL, key string) importResponse {
	suite.T().Helper()
	return suite.importDocument(suite.gatewayDocument(name, baseURL, key, ""), runtimeOptions(), false)
}

// gatewayDocument builds a gateway document, omitting the fields left empty.
func (suite *ImportGatewaySuite) gatewayDocument(name, baseURL, key, caCertificate string) string {
	content := fmt.Sprintf("resource_type: gateway\nname: %s\nbaseUrl: %s\n", name, baseURL)
	if key != "" {
		content += fmt.Sprintf("key: %s\n", key)
	}
	if caCertificate != "" {
		content += fmt.Sprintf("caCertificate: %q\n", caCertificate)
	}
	return content
}

// importDocument posts one document and returns the decoded response.
func (suite *ImportGatewaySuite) importDocument(
	content string, options importOptions, dryRun bool) importResponse {
	suite.T().Helper()

	status, body := suite.post("/import", suite.marshal(importRequest{
		Content: content, Options: options, DryRun: dryRun,
	}))
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var resp importResponse
	suite.Require().NoError(json.Unmarshal(body, &resp))
	return resp
}

// countGatewaysNamed reports how many registered gateways carry that name.
func (suite *ImportGatewaySuite) countGatewaysNamed(name string) int {
	suite.T().Helper()

	status, body := suite.get("/gateways")
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var listed []struct {
		Name string `json:"name"`
	}
	suite.Require().NoError(json.Unmarshal(body, &listed))

	count := 0
	for _, entry := range listed {
		if entry.Name == name {
			count++
		}
	}
	return count
}

func (suite *ImportGatewaySuite) marshal(req importRequest) string {
	suite.T().Helper()
	payload, err := json.Marshal(req)
	suite.Require().NoError(err)
	return string(payload)
}

func (suite *ImportGatewaySuite) post(path, body string) (int, []byte) {
	suite.T().Helper()
	return suite.do(http.MethodPost, path, body)
}

func (suite *ImportGatewaySuite) get(path string) (int, []byte) {
	suite.T().Helper()
	return suite.do(http.MethodGet, path, "")
}

func (suite *ImportGatewaySuite) do(method, path, body string) (int, []byte) {
	suite.T().Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}

	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	suite.Require().NoError(err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, payload
}

// A dry run reports what would happen and registers nothing, for a new gateway and for one that is
// already registered alike.
func (suite *ImportGatewaySuite) TestDryRunRegistersNothing() {
	name := fmt.Sprintf("dryrun-gateway-%d", time.Now().UnixNano())
	document := suite.gatewayDocument(name, "https://"+name+".test:8090", "the-imported-key", "")

	resp := suite.importDocument(document, runtimeOptions(), true)

	suite.Require().Len(resp.Results, 1)
	suite.Equal("success", resp.Results[0].Status)
	suite.Equal("create", resp.Results[0].Operation, "a new gateway should be reported as a create")
	suite.Equal(0, suite.countGatewaysNamed(name), "a dry run registered a gateway")

	// Now register it, and a dry run over the same document reports an update instead.
	created := suite.importGateway(name, "https://"+name+".test:8090", "the-imported-key")
	suite.Require().Equal(1, created.Summary.Imported, "results: %+v", created.Results)
	defer suite.removeGateway(created.Results[0].ResourceID)

	again := suite.importDocument(document, runtimeOptions(), true)
	suite.Require().Len(again.Results, 1)
	suite.Equal("update", again.Results[0].Operation, "an existing gateway should be reported as an update")
	suite.Equal(1, suite.countGatewaysNamed(name), "a dry run registered a second gateway")
}

// With upsert turned off, a name already registered is a conflict rather than a silent replacement.
func (suite *ImportGatewaySuite) TestImportRefusesADuplicateNameWithoutUpsert() {
	name := fmt.Sprintf("duplicate-gateway-%d", time.Now().UnixNano())
	first := suite.importGateway(name, "https://"+name+".test:8090", "the-imported-key")
	suite.Require().Equal(1, first.Summary.Imported, "results: %+v", first.Results)
	defer suite.removeGateway(first.Results[0].ResourceID)

	noUpsert := importOptions{Upsert: false, ContinueOnError: true, Target: "runtime"}
	second := suite.importDocument(
		suite.gatewayDocument(name, "https://"+name+".other.test:8090", "", ""), noUpsert, false)

	suite.Require().Len(second.Results, 1)
	suite.Equal("failed", second.Results[0].Status, "a duplicate name was accepted: %+v", second.Results)
	suite.Equal(1, suite.countGatewaysNamed(name), "the refused document still registered a gateway")
}

// A re-import that names a key and a certificate authority applies both, which is the other half of
// the update path: the fields a document omits are left alone, and the ones it carries are not.
func (suite *ImportGatewaySuite) TestReimportAppliesTheFieldsItCarries() {
	name := fmt.Sprintf("rotating-gateway-%d", time.Now().UnixNano())
	first := suite.importGateway(name, "https://"+name+".test:8090", "the-first-key")
	suite.Require().Equal(1, first.Summary.Imported, "results: %+v", first.Results)
	id := first.Results[0].ResourceID
	defer suite.removeGateway(id)

	const authority = "-----BEGIN CERTIFICATE-----\nnot-a-real-certificate\n-----END CERTIFICATE-----"
	second := suite.importDocument(
		suite.gatewayDocument(name, "https://"+name+".test:8090", "the-rotated-key", authority),
		runtimeOptions(), false)

	suite.Require().Equal(1, second.Summary.Imported, "results: %+v", second.Results)
	suite.Equal(id, second.Results[0].ResourceID)

	status, body := suite.get("/gateways/" + id)
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	var registered struct {
		CACertificate string `json:"caCertificate"`
	}
	suite.Require().NoError(json.Unmarshal(body, &registered))
	suite.Equal(authority, registered.CACertificate, "the certificate authority was not applied")

	// The rotated key is stored but still never readable.
	suite.NotContains(string(body), "the-rotated-key", "the rotated key was returned by a read")
}
