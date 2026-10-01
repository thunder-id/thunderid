// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package variablestore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = "https://localhost:8095"

// variable is what a read of one returns: the value comes back.
type variable struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// secret is what a read of one returns. There is no value field: a secret is written and
// referenced, never read back.
type secret struct {
	Name        string `json:"name"`
	Exists      bool   `json:"exists"`
	Description string `json:"description,omitempty"`
}

type variableList struct {
	TotalResults int        `json:"totalResults"`
	StartIndex   int        `json:"startIndex"`
	Count        int        `json:"count"`
	Variables    []variable `json:"variables"`
}

type secretList struct {
	TotalResults int      `json:"totalResults"`
	Count        int      `json:"count"`
	Secrets      []secret `json:"secrets"`
}

type errorResponse struct {
	Code string `json:"code"`
}

type VariableStoreAPITestSuite struct {
	suite.Suite
	// prefix namespaces every name this run creates.
	//
	// Fixed names would make the suite depend on the store being empty: a run interrupted before its
	// teardown leaves names behind, and the next run then meets a 409 it did not expect, or counts
	// more matches than it created. Neither is a fault in the code under test, which is the worst
	// kind of failing test. A prefix per run means a leftover from an earlier one is invisible here.
	prefix    string
	variables []string
	secrets   []string
}

// SetupSuite picks the namespace this run works in.
func (ts *VariableStoreAPITestSuite) SetupSuite() {
	ts.prefix = fmt.Sprintf("INT_%d_", time.Now().UnixNano())
}

// name qualifies a name with this run's prefix.
func (ts *VariableStoreAPITestSuite) name(suffix string) string {
	return ts.prefix + suffix
}

func TestVariableStoreAPITestSuite(t *testing.T) {
	suite.Run(t, new(VariableStoreAPITestSuite))
}

// TearDownTest removes what a test stored, so one test's names do not appear in another's listing.
func (ts *VariableStoreAPITestSuite) TearDownTest() {
	for _, name := range ts.variables {
		ts.do(http.MethodDelete, "/variables/"+name, "")
	}
	for _, name := range ts.secrets {
		ts.do(http.MethodDelete, "/secrets/"+name, "")
	}
	ts.variables, ts.secrets = nil, nil
}

func (ts *VariableStoreAPITestSuite) TestCreateReadAndDeleteAVariable() {
	ts.createVariable(ts.name("API_URL"), "https://example.test", "where the api answers")

	fetched := ts.getVariable(ts.name("API_URL"), http.StatusOK)
	ts.Equal("https://example.test", fetched.Value)
	ts.Equal("where the api answers", fetched.Description)
	// The database assigns these; empty means the response was built rather than read back.
	ts.NotEmpty(fetched.CreatedAt)

	ts.Equal(http.StatusNoContent, ts.do(http.MethodDelete, "/variables/"+ts.name("API_URL"), ""))
	ts.variables = nil

	ts.getVariable(ts.name("API_URL"), http.StatusNotFound)
}

// Creating the same name twice is a conflict, not a silent overwrite.
func (ts *VariableStoreAPITestSuite) TestCreatingAVariableTwiceConflicts() {
	ts.createVariable(ts.name("DUPLICATE"), "first", "")

	status, body := ts.postWithBody("/variables",
		fmt.Sprintf(`{"name":%q,"value":"second"}`, ts.name("DUPLICATE")))

	ts.Equal(http.StatusConflict, status)
	ts.Equal("VAR-1009", codeOf(ts.T(), body))

	// The first value is still there: a refused create changed nothing.
	ts.Equal("first", ts.getVariable(ts.name("DUPLICATE"), http.StatusOK).Value)
}

// PUT is create-or-replace: it creates when the name is unused and replaces when it is used, and
// the status says which happened.
func (ts *VariableStoreAPITestSuite) TestPutCreatesThenReplaces() {
	status, _ := ts.putWithBody("/variables/"+ts.name("PUT"), `{"value":"created"}`)
	ts.Equal(http.StatusCreated, status)
	ts.variables = append(ts.variables, ts.name("PUT"))

	status, _ = ts.putWithBody("/variables/"+ts.name("PUT"), `{"value":"replaced"}`)
	ts.Equal(http.StatusOK, status)

	ts.Equal("replaced", ts.getVariable(ts.name("PUT"), http.StatusOK).Value)
}

// Deleting is idempotent, so a caller cleaning up does not have to ask first.
func (ts *VariableStoreAPITestSuite) TestDeletingSomethingThatIsNotStored() {
	ts.Equal(http.StatusNoContent, ts.do(http.MethodDelete, "/variables/"+ts.name("NEVER_STORED"), ""))
	ts.Equal(http.StatusNoContent, ts.do(http.MethodDelete, "/secrets/"+ts.name("NEVER_STORED"), ""))
}

// A secret is written and referenced. No endpoint returns its value.
func (ts *VariableStoreAPITestSuite) TestASecretValueIsNeverReturned() {
	const value = "a-very-recognizable-secret-value"
	ts.createSecret(ts.name("DB_PASSWORD"), value)

	for _, path := range []string{"/secrets/" + ts.name("DB_PASSWORD"), "/secrets"} {
		body := ts.rawGet(path)
		ts.NotContains(body, value, "GET %s returned the secret's value", path)
		ts.NotContains(body, `"value"`, "GET %s carried a value field", path)
	}

	// What a read does report is that one is held.
	var held secret
	ts.Require().NoError(json.Unmarshal([]byte(ts.rawGet("/secrets/"+ts.name("DB_PASSWORD"))), &held))
	ts.True(held.Exists)
}

// Rotation replaces the value without ever reading the old one.
func (ts *VariableStoreAPITestSuite) TestASecretCanBeRotated() {
	ts.createSecret(ts.name("ROTATING"), "first-value")

	status, body := ts.putWithBody("/secrets/"+ts.name("ROTATING"), `{"value":"second-value"}`)

	ts.Equal(http.StatusOK, status)
	ts.NotContains(body, "second-value", "the rotation echoed the new value back")
}

func (ts *VariableStoreAPITestSuite) TestAnEmptySecretIsRefused() {
	status, body := ts.postWithBody("/secrets", fmt.Sprintf(`{"name":%q,"value":""}`, ts.name("EMPTY")))

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("VAR-1003", codeOf(ts.T(), body))
}

func (ts *VariableStoreAPITestSuite) TestAnInvalidNameIsRefused() {
	for _, name := range []string{"has space", "1leading", "dash-ed"} {
		status, body := ts.postWithBody("/variables",
			fmt.Sprintf(`{"name":%q,"value":"v"}`, name))

		ts.Equal(http.StatusBadRequest, status, "a name of %q was accepted", name)
		ts.Equal("VAR-1002", codeOf(ts.T(), body))
	}
}

func (ts *VariableStoreAPITestSuite) TestListingIsPagedAndFiltered() {
	ts.createVariable(ts.name("LIST_ALPHA"), "a", "")
	ts.createVariable(ts.name("LIST_BETA"), "b", "")

	listed := ts.listVariables("?filter=" + urlEncode(fmt.Sprintf("name sw %q", ts.name("LIST_"))))
	ts.Equal(2, listed.TotalResults)

	// A page carries at most what was asked for, and reports the total beyond it.
	page := ts.listVariables("?limit=1&filter=" + urlEncode(fmt.Sprintf("name sw %q", ts.name("LIST_"))))
	ts.Equal(1, page.Count)
	ts.Equal(2, page.TotalResults)

	// Asking which of a set exist returns only those that do.
	named := ts.listVariables(fmt.Sprintf("?names=%s,%s", ts.name("LIST_ALPHA"), ts.name("NOT_STORED")))
	ts.Equal(1, named.TotalResults)
	ts.Require().Len(named.Variables, 1)
	ts.Equal(ts.name("LIST_ALPHA"), named.Variables[0].Name)
}

// A query this store does not understand is refused rather than answered with everything.
func (ts *VariableStoreAPITestSuite) TestAnUnreadableQueryIsRefused() {
	for _, query := range []string{
		"?filter=" + urlEncode("name eq unquoted"),
		"?filter=",
		"?sort=name",
		"?limit=0",
		"?limit=1000",
	} {
		status, _ := ts.get("/variables" + query)
		ts.Equal(http.StatusBadRequest, status, "the query %q was accepted", query)
	}
}

func (ts *VariableStoreAPITestSuite) TestSecretsAreListedByNameOnly() {
	ts.createSecret(ts.name("LISTED_SECRET"), "value")

	body := ts.rawGet("/secrets")
	var listed secretList
	ts.Require().NoError(json.Unmarshal([]byte(body), &listed))

	found := false
	for _, s := range listed.Secrets {
		if s.Name == ts.name("LISTED_SECRET") {
			found = true
			ts.True(s.Exists)
		}
	}
	ts.True(found, "the stored secret was not listed")
}

// ---- helpers ----

func (ts *VariableStoreAPITestSuite) createVariable(name, value, description string) {
	ts.T().Helper()

	status, _ := ts.postWithBody("/variables",
		fmt.Sprintf(`{"name":%q,"value":%q,"description":%q}`, name, value, description))
	ts.Require().Equal(http.StatusCreated, status)
	ts.variables = append(ts.variables, name)
}

func (ts *VariableStoreAPITestSuite) createSecret(name, value string) {
	ts.T().Helper()

	status, _ := ts.postWithBody("/secrets", fmt.Sprintf(`{"name":%q,"value":%q}`, name, value))
	ts.Require().Equal(http.StatusCreated, status)
	ts.secrets = append(ts.secrets, name)
}

func (ts *VariableStoreAPITestSuite) getVariable(name string, wantStatus int) variable {
	ts.T().Helper()

	status, body := ts.get("/variables/" + name)
	ts.Require().Equal(wantStatus, status)

	var fetched variable
	if wantStatus == http.StatusOK {
		ts.Require().NoError(json.Unmarshal([]byte(body), &fetched))
	}
	return fetched
}

func (ts *VariableStoreAPITestSuite) listVariables(query string) variableList {
	ts.T().Helper()

	status, body := ts.get("/variables" + query)
	ts.Require().Equal(http.StatusOK, status, "listing failed: %s", body)

	var listed variableList
	ts.Require().NoError(json.Unmarshal([]byte(body), &listed))
	return listed
}

func (ts *VariableStoreAPITestSuite) postWithBody(path, body string) (int, string) {
	return ts.send(http.MethodPost, path, body)
}

func (ts *VariableStoreAPITestSuite) putWithBody(path, body string) (int, string) {
	return ts.send(http.MethodPut, path, body)
}

func (ts *VariableStoreAPITestSuite) get(path string) (int, string) {
	return ts.send(http.MethodGet, path, "")
}

func (ts *VariableStoreAPITestSuite) rawGet(path string) string {
	_, body := ts.send(http.MethodGet, path, "")
	return body
}

func (ts *VariableStoreAPITestSuite) do(method, path, body string) int {
	status, _ := ts.send(method, path, body)
	return status
}

func (ts *VariableStoreAPITestSuite) send(method, path, body string) (int, string) {
	ts.T().Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	ts.Require().NoError(err)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(raw)
}

func codeOf(t *testing.T, body string) string {
	t.Helper()

	var failure errorResponse
	if err := json.Unmarshal([]byte(body), &failure); err != nil {
		t.Fatalf("the error response was unreadable: %s", body)
	}
	return failure.Code
}

// urlEncode escapes a filter expression for a query string.
func urlEncode(expression string) string {
	return url.QueryEscape(expression)
}

// A secret created by PUT is a creation, and rotating it afterwards is a replacement. The status
// tells them apart, which is what makes PUT usable without asking first.
func (ts *VariableStoreAPITestSuite) TestPuttingASecretCreatesThenRotates() {
	status, _ := ts.putWithBody("/secrets/"+ts.name("PUT_SECRET"), `{"value":"first"}`)
	ts.Equal(http.StatusCreated, status)
	ts.secrets = append(ts.secrets, ts.name("PUT_SECRET"))

	status, _ = ts.putWithBody("/secrets/"+ts.name("PUT_SECRET"), `{"value":"second"}`)
	ts.Equal(http.StatusOK, status)
}

// A name that is not stored is reported as such, for both collections.
func (ts *VariableStoreAPITestSuite) TestReadingSomethingThatIsNotStored() {
	status, body := ts.get("/variables/" + ts.name("ABSENT"))
	ts.Equal(http.StatusNotFound, status)
	ts.Equal("VAR-1004", codeOf(ts.T(), body))

	status, body = ts.get("/secrets/" + ts.name("ABSENT"))
	ts.Equal(http.StatusNotFound, status)
	ts.Equal("VAR-1004", codeOf(ts.T(), body))
}

// Creating a secret whose name is taken is a conflict, not a silent rotation.
func (ts *VariableStoreAPITestSuite) TestCreatingASecretTwiceConflicts() {
	ts.createSecret(ts.name("SECRET_DUPLICATE"), "first")

	status, body := ts.postWithBody("/secrets",
		fmt.Sprintf(`{"name":%q,"value":"second"}`, ts.name("SECRET_DUPLICATE")))

	ts.Equal(http.StatusConflict, status)
	ts.Equal("VAR-1009", codeOf(ts.T(), body))
}

// A description travels with the value, and is returned with it.
func (ts *VariableStoreAPITestSuite) TestADescriptionIsStored() {
	ts.createVariable(ts.name("DESCRIBED"), "v", "what this is for")

	ts.Equal("what this is for", ts.getVariable(ts.name("DESCRIBED"), http.StatusOK).Description)

	// A secret carries one too, and it is not a secret: only the value is withheld.
	status, _ := ts.postWithBody("/secrets",
		fmt.Sprintf(`{"name":%q,"value":"v","description":"the database password"}`, ts.name("DESCRIBED_SECRET")))
	ts.Require().Equal(http.StatusCreated, status)
	ts.secrets = append(ts.secrets, ts.name("DESCRIBED_SECRET"))

	ts.Contains(ts.rawGet("/secrets/"+ts.name("DESCRIBED_SECRET")), "the database password")
}

// Paging carries the narrowing forward, so following a page does not widen the result.
func (ts *VariableStoreAPITestSuite) TestAPageLinksToTheNextWithoutWidening() {
	for _, name := range []string{ts.name("PAGE_A"), ts.name("PAGE_B"), ts.name("PAGE_C")} {
		ts.createVariable(name, "v", "")
	}

	status, body := ts.get("/variables?limit=2&filter=" + urlEncode(fmt.Sprintf("name sw %q", ts.name("PAGE_"))))
	ts.Require().Equal(http.StatusOK, status)

	var page struct {
		TotalResults int `json:"totalResults"`
		Count        int `json:"count"`
		Links        []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"links"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &page))

	ts.Equal(3, page.TotalResults)
	ts.Equal(2, page.Count)
	ts.Require().NotEmpty(page.Links, "a page with more behind it carried no next link")
	ts.Equal("next", page.Links[0].Rel)
	ts.Contains(page.Links[0].Href, "filter=", "the next link dropped the narrowing")
}

// An exact-match filter returns the one named, and nothing near it.
func (ts *VariableStoreAPITestSuite) TestAnExactFilterMatchesOne() {
	ts.createVariable(ts.name("EXACT"), "v", "")
	ts.createVariable(ts.name("EXACT_LONGER"), "v", "")

	listed := ts.listVariables("?filter=" + urlEncode(fmt.Sprintf("name eq %q", ts.name("EXACT"))))

	ts.Equal(1, listed.TotalResults)
	ts.Require().Len(listed.Variables, 1)
	ts.Equal(ts.name("EXACT"), listed.Variables[0].Name)
}

// A value longer than the store accepts is refused, as is a description.
func (ts *VariableStoreAPITestSuite) TestOversizedInputIsRefused() {
	long := strings.Repeat("v", 8193)
	status, body := ts.postWithBody("/variables",
		fmt.Sprintf(`{"name":%q,"value":%q}`, ts.name("TOO_LONG"), long))
	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("VAR-1003", codeOf(ts.T(), body))

	status, body = ts.postWithBody("/variables",
		fmt.Sprintf(`{"name":%q,"value":"v","description":%q}`, ts.name("LONG_DESC"), strings.Repeat("d", 1001)))
	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("VAR-1008", codeOf(ts.T(), body))
}

// Asking for more names than the store will look up at once is refused rather than truncated.
func (ts *VariableStoreAPITestSuite) TestTooManyNamesIsRefused() {
	names := make([]string, 101)
	for i := range names {
		names[i] = fmt.Sprintf("%sNAME_%d", ts.prefix, i)
	}

	status, body := ts.get("/variables?names=" + strings.Join(names, ","))

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("VAR-1007", codeOf(ts.T(), body))
}
