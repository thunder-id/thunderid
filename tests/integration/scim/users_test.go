/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package scim

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SCIMUsersTestSuite exercises the full Users lifecycle a real SCIM
// provisioning connector drives: discover the schema, create, read, list by
// filter, replace, and delete. It also covers the two error paths most
// relevant to real provisioning traffic: a payload that satisfies core fields
// but not the usertype's own required attributes, and a duplicate unique
// value.
//
// ThunderID does not require any usertype to declare a "username" attribute —
// userName is only persisted/enforced if the usertype's own schema declares a
// matching property, same as any other core-mapped field. This fixture's
// unique identifier is "email" instead (required, core-mapped, and unique),
// which is at least as common a real-world usertype shape as username-based
// identity. "department" is an optional, extension-only attribute — this
// exercises both the core-attribute reverse-mapping path and plain
// extension-object attributes in the same fixture.
type SCIMUsersTestSuite struct {
	suite.Suite
	ouID           string
	entityTypeID   string
	entityTypeName string
	extensionURN   string
	createdUserIDs []string

	// A second, minimal usertype used only by TestReplaceUserImmutableTypeChangeRejected
	// to attempt swapping a user's type via PUT.
	altEntityTypeID   string
	altEntityTypeName string
	altExtensionURN   string
}

func TestSCIMUsersTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMUsersTestSuite))
}

func (ts *SCIMUsersTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-users-ou",
		Name:        "SCIM Users Integration Test OU",
		Description: "Organization unit for SCIM Users endpoint tests",
	})
	ts.Require().NoError(err, "failed to create test organization unit")
	ts.ouID = ouID

	ts.entityTypeName = "scim-it-users-person"
	entityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.entityTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"email":      map[string]interface{}{"type": "string", "required": true, "unique": true},
			"department": map[string]interface{}{"type": "string"},
			"team":       map[string]interface{}{"type": "string"},
			"status":     map[string]interface{}{"type": "string", "enum": []string{"active", "inactive"}},
			"level":      map[string]interface{}{"type": "number", "enum": []int{1, 2, 3}},
		},
	})
	ts.Require().NoError(err, "failed to create test entity type")
	ts.entityTypeID = entityTypeID

	urn, required, err := discoverExtensionSchema(ts.entityTypeName)
	ts.Require().NoError(err, "failed to discover extension schema via GET /Schemas")
	ts.Require().Contains(required, "email", "email should be discoverable as required before any user is created")
	ts.extensionURN = urn

	ts.altEntityTypeName = "scim-it-users-person-alt"
	altEntityTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: ts.altEntityTypeName,
		OUID: ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "failed to create alt test entity type")
	ts.altEntityTypeID = altEntityTypeID

	altURN, _, err := discoverExtensionSchema(ts.altEntityTypeName)
	ts.Require().NoError(err, "failed to discover alt extension schema via GET /Schemas")
	ts.altExtensionURN = altURN
}

func (ts *SCIMUsersTestSuite) TearDownSuite() {
	for _, id := range ts.createdUserIDs {
		_, _, _ = scimRequest(http.MethodDelete, "/Users/"+id, nil, nil)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.altEntityTypeID != "" {
		_ = testutils.DeleteUserType(ts.altEntityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// buildUserBody constructs a SCIM User create/replace payload. userName is
// always sent (SCIM requires it on the wire) but this usertype has no
// matching schema property, so it is never persisted — that's expected, not
// a bug. When email is non-empty, the top-level "emails" field is populated
// so the core-mapped, required "email" attribute is satisfiable via
// reverse-mapping, mirroring what a real client sends for a core-mapped
// field.
func (ts *SCIMUsersTestSuite) buildUserBody(username, email string, extAttrs map[string]interface{}) []byte {
	if extAttrs == nil {
		extAttrs = map[string]interface{}{}
	}
	payload := map[string]interface{}{
		"schemas":       []string{scimCoreUserSchemaURN, ts.extensionURN},
		"userName":      username,
		ts.extensionURN: extAttrs,
	}
	if email != "" {
		payload["emails"] = []map[string]interface{}{
			{"value": email, "type": "work"},
		}
	}
	b, err := json.Marshal(payload)
	ts.Require().NoError(err)
	return b
}

func (ts *SCIMUsersTestSuite) createUser(username, email string, extAttrs map[string]interface{}) (int, map[string]interface{}) {
	status, body, err := scimRequest(http.MethodPost, "/Users", ts.buildUserBody(username, email, extAttrs), nil)
	ts.Require().NoError(err)

	var resp map[string]interface{}
	if len(body) > 0 {
		ts.Require().NoError(json.Unmarshal(body, &resp))
	}
	if status == http.StatusCreated {
		id, _ := resp["id"].(string)
		ts.createdUserIDs = append(ts.createdUserIDs, id)
	}
	return status, resp
}

// firstEmailValue extracts the "value" of the first entry in a decoded
// response's top-level "emails" array (the core-mapped SCIM representation
// of the schema's "email" attribute).
func firstEmailValue(resp map[string]interface{}) (string, bool) {
	emails, ok := resp["emails"].([]interface{})
	if !ok || len(emails) == 0 {
		return "", false
	}
	entry, ok := emails[0].(map[string]interface{})
	if !ok {
		return "", false
	}
	v, ok := entry["value"].(string)
	return v, ok
}

func (ts *SCIMUsersTestSuite) TestCreateAndGetUser() {
	email := "scim.it.create@example.com"
	status, created := ts.createUser("scim.it.create", email, nil)
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %v", created)

	gotEmail, ok := firstEmailValue(created)
	ts.Require().True(ok, "response should include the core-mapped emails field")
	ts.Equal(email, gotEmail)
	ts.Contains(created, ts.extensionURN, "response should embed the extension object under its URN key")

	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var fetched map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ts.Equal(id, fetched["id"])
	gotEmail, ok = firstEmailValue(fetched)
	ts.Require().True(ok)
	ts.Equal(email, gotEmail)
}

func (ts *SCIMUsersTestSuite) TestListAndFilterByEmail() {
	email := "scim.it.filter@example.com"
	status, created := ts.createUser("scim.it.filter", email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	filter := url.QueryEscape(`emails.value eq "` + email + `"`)
	status, body, err := scimRequest(http.MethodGet, "/Users?filter="+filter, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list scimUserListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))
	ts.Require().Equal(1, list.TotalResults, "filter should match exactly the one user created for this test")
	ts.Equal(id, list.Resources[0].ID)
}

func (ts *SCIMUsersTestSuite) TestReplaceUserUpdatesExtensionAttribute() {
	email := "scim.it.replace@example.com"
	status, created := ts.createUser("scim.it.replace", email, map[string]interface{}{"department": "Support"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	// PUT is a full replace: the required core-mapped field must be resent.
	replaceBody := ts.buildUserBody("scim.it.replace", email, map[string]interface{}{"department": "Engineering"})
	status, body, err := scimRequest(http.MethodPut, "/Users/"+id, replaceBody, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "replace failed: %s", body)

	status, body, err = scimRequest(http.MethodGet, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var fetched map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ext, ok := fetched[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok, "response should contain the extension object")
	ts.Equal("Engineering", ext["department"])
}

// TestMissingRequiredExtensionAttribute is the integration-level check for the
// missingRequiredAttrs fix: core only requires nothing of its own, but this
// usertype requires "email" — sending userName with no emails must fail with
// a 400 naming the missing attribute, not a generic schema-mismatch error.
func (ts *SCIMUsersTestSuite) TestMissingRequiredExtensionAttribute() {
	status, resp := ts.createUser("scim.it.missing-required", "", nil)
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)

	body, err := json.Marshal(resp)
	ts.Require().NoError(err)
	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
	ts.Contains(errResp.Detail, "email", "detail should name the missing attribute so the client can act on it")
}

// TestDuplicateEmailConflict uses email, not userName, as the collision
// target: this usertype's schema marks "email" unique, not username (which
// isn't even declared) — uniqueness in ThunderID is whatever the usertype
// schema says it is, not a hardcoded userName assumption.
func (ts *SCIMUsersTestSuite) TestDuplicateEmailConflict() {
	email := "scim.it.duplicate@example.com"
	status, _ := ts.createUser("scim.it.duplicate.a", email, nil)
	ts.Require().Equal(http.StatusCreated, status)

	status, resp := ts.createUser("scim.it.duplicate.b", email, nil)
	ts.Require().Equal(http.StatusConflict, status, "expected 409, got body: %v", resp)

	body, err := json.Marshal(resp)
	ts.Require().NoError(err)
	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("uniqueness", errResp.ScimType)
}

func (ts *SCIMUsersTestSuite) TestGetUnknownUserReturns404() {
	status, _, err := scimRequest(http.MethodGet, "/Users/does-not-exist", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status)
}

func (ts *SCIMUsersTestSuite) TestDeleteUserThenGetReturns404() {
	status, created := ts.createUser("scim.it.delete", "scim.it.delete@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodDelete, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusNoContent, status)

	status, _, err = scimRequest(http.MethodGet, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status, "user should be gone after delete")

	// Already deleted — drop it from cleanup so TearDownSuite doesn't retry.
	for i, cid := range ts.createdUserIDs {
		if cid == id {
			ts.createdUserIDs = append(ts.createdUserIDs[:i], ts.createdUserIDs[i+1:]...)
			break
		}
	}
}

// ---------------------------------------------------------------------------
// Request-shape edge cases
// ---------------------------------------------------------------------------

func (ts *SCIMUsersTestSuite) TestCreateUserWrongContentTypeRejected() {
	body := ts.buildUserBody("scim.it.wrong-content-type", "scim.it.wrong-content-type@example.com", nil)
	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

func (ts *SCIMUsersTestSuite) TestCreateUserMalformedJSONRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users", []byte(`{not valid json`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

func (ts *SCIMUsersTestSuite) TestCreateUserMissingSchemasRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"userName":      "scim.it.missing-schemas",
		ts.extensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a request with no \"schemas\" array must be rejected")
}

func (ts *SCIMUsersTestSuite) TestCreateUserDuplicateSchemaURNsRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":       []string{scimCoreUserSchemaURN, ts.extensionURN, scimCoreUserSchemaURN},
		"userName":      "scim.it.duplicate-schema-urn",
		"emails":        []map[string]interface{}{{"value": "scim.it.duplicate-schema-urn@example.com"}},
		ts.extensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "duplicate URNs in \"schemas\" must be rejected")
}

func (ts *SCIMUsersTestSuite) TestCreateUserTwoExtensionURNsRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":          []string{scimCoreUserSchemaURN, ts.extensionURN, ts.altExtensionURN},
		"userName":         "scim.it.two-extension-urns",
		"emails":           []map[string]interface{}{{"value": "scim.it.two-extension-urns@example.com"}},
		ts.extensionURN:    map[string]interface{}{},
		ts.altExtensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "more than one ThunderID extension URN must be rejected")
}

// TestCreateUserUndeclaredAttributeRejected pins undeclaredAttrs (schema_builder.go):
// an extension attribute this usertype's schema does not declare must be rejected
// with a message naming it, not silently accepted or silently dropped.
func (ts *SCIMUsersTestSuite) TestCreateUserUndeclaredAttributeRejected() {
	status, resp := ts.createUser("scim.it.undeclared-attr", "scim.it.undeclared-attr@example.com",
		map[string]interface{}{"nonexistent_field": "x"})
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)

	body, err := json.Marshal(resp)
	ts.Require().NoError(err)
	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
	ts.Contains(errResp.Detail, "nonexistent_field")
}

func (ts *SCIMUsersTestSuite) TestCreateUserInvalidStringEnumRejected() {
	status, resp := ts.createUser("scim.it.invalid-string-enum", "scim.it.invalid-string-enum@example.com",
		map[string]interface{}{"status": "not-a-declared-enum-value"})
	ts.Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)
}

func (ts *SCIMUsersTestSuite) TestCreateUserInvalidNumberEnumRejected() {
	status, resp := ts.createUser("scim.it.invalid-number-enum", "scim.it.invalid-number-enum@example.com",
		map[string]interface{}{"level": 99})
	ts.Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)
}

// ---------------------------------------------------------------------------
// Replace edge cases
// ---------------------------------------------------------------------------

// TestReplaceUserImmutableTypeChangeRejected pins that PUT cannot change a
// user's type (schema extension) — the extension URN in a replace request
// must match the target resource's existing one.
func (ts *SCIMUsersTestSuite) TestReplaceUserImmutableTypeChangeRejected() {
	email := "scim.it.immutable-type@example.com"
	status, created := ts.createUser("scim.it.immutable-type", email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":          []string{scimCoreUserSchemaURN, ts.altExtensionURN},
		"userName":         "scim.it.immutable-type",
		"emails":           []map[string]interface{}{{"value": email}},
		ts.altExtensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPut, "/Users/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "replacing with a different usertype's extension URN must be rejected")
}

func (ts *SCIMUsersTestSuite) TestReplaceUserIfMatchMismatchRejected() {
	email := "scim.it.if-match@example.com"
	status, created := ts.createUser("scim.it.if-match", email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	replaceBody := ts.buildUserBody("scim.it.if-match", email, nil)
	status, _, err := scimRequest(http.MethodPut, "/Users/"+id, replaceBody,
		map[string]string{"If-Match": `"bogus-etag"`})
	ts.Require().NoError(err)
	ts.Equal(http.StatusPreconditionFailed, status)
}

// ---------------------------------------------------------------------------
// Attribute projection (RFC 7644 §3.9)
// ---------------------------------------------------------------------------

func (ts *SCIMUsersTestSuite) TestGetUserAttributesProjection() {
	email := "scim.it.projection.attrs@example.com"
	status, created := ts.createUser("scim.it.projection.attrs", email,
		map[string]interface{}{"department": "Engineering", "team": "Blue"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id+"?attributes=department", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var projected map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &projected))

	ts.Contains(projected, "id")
	ts.Contains(projected, "schemas")
	ts.Contains(projected, "meta")
	ts.NotContains(projected, "emails", "attributes=department should not implicitly keep top-level emails")

	ext, ok := projected[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok, "extension object should still be present, holding only the requested attribute")
	ts.Contains(ext, "department")
	ts.NotContains(ext, "team", "attributes=department should drop the unrequested extension attribute")
}

func (ts *SCIMUsersTestSuite) TestGetUserExcludedAttributesProjection() {
	email := "scim.it.projection.excluded@example.com"
	status, created := ts.createUser("scim.it.projection.excluded", email,
		map[string]interface{}{"department": "Engineering", "team": "Blue"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id+"?excludedAttributes=team", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var projected map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &projected))

	ext, ok := projected[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok, "extension object should still be present")
	ts.NotContains(ext, "team", "excludedAttributes=team should drop team")
	ts.Contains(ext, "department", "excludedAttributes=team must keep the rest of the extension attributes")
}
