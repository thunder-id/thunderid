// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// scimEnterpriseUserSchemaURN mirrors backend/internal/scim's SCIMEnterpriseUserSchemaURN.
const scimEnterpriseUserSchemaURN = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"

// SCIMUsersTestSuite exercises the full Users lifecycle a real SCIM
// provisioning connector drives: discover the schema, create, read, list by
// filter, replace, and delete. It also covers the two error paths most
// relevant to real provisioning traffic: a payload that satisfies core fields
// but not the usertype's own required attributes, and a duplicate unique
// value.
//
// Only the designated core user type (the SCIM core user type, the declarative
// "decl-schema-1" here) may carry the core User schema, so this suite's own
// usertype is extension-only: its unique identifier is the required, unique
// "email" extension attribute. The core-schema path itself is covered against the
// designated type by TestCreateCoreSchemaUserOnDesignatedType, and its rejection
// on any other type by TestCoreSchemaRejectedForNonDesignatedType.
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

	// Extension URN of the designated core user type (SCIM core user type).
	coreExtensionURN string
}

const scimCoreUserTypeName = "declarative-test-schema"

func TestSCIMUsersTestSuite(t *testing.T) {
	suite.Run(t, new(SCIMUsersTestSuite))
}

// SetupSuite initializes the test suite environment.
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
		Handle:      ts.entityTypeName,
		DisplayName: "Entity Type",
		OUID:        ouID,
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
		Handle:      ts.altEntityTypeName,
		DisplayName: "Alt Entity Type",
		OUID:        ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "failed to create alt test entity type")
	ts.altEntityTypeID = altEntityTypeID

	altURN, _, err := discoverExtensionSchema(ts.altEntityTypeName)
	ts.Require().NoError(err, "failed to discover alt extension schema via GET /Schemas")
	ts.altExtensionURN = altURN

	coreURN, _, err := discoverExtensionSchema(scimCoreUserTypeName)
	ts.Require().NoError(err, "failed to discover the designated core user type's extension schema")
	ts.coreExtensionURN = coreURN
}

// TearDownSuite cleans up the test suite environment.
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

// buildUserBody constructs an extension-only SCIM User create/replace payload. This usertype
// is not the designated core user type, so the payload carries no core User schema or
// top-level core attributes (userName, emails); email is a plain extension attribute. When
// email is empty it is omitted, leaving the required attribute unsatisfied.
// buildUserBody handles build user body.
func (ts *SCIMUsersTestSuite) buildUserBody(email string, extAttrs map[string]interface{}) []byte {
	ext := map[string]interface{}{}
	for k, v := range extAttrs {
		ext[k] = v
	}
	if email != "" {
		ext["email"] = email
	}
	payload := map[string]interface{}{
		"schemas":       []string{ts.extensionURN},
		ts.extensionURN: ext,
	}
	b, err := json.Marshal(payload)
	ts.Require().NoError(err)
	return b
}

// createUser handles create user.
func (ts *SCIMUsersTestSuite) createUser(email string, extAttrs map[string]interface{}) (int, map[string]interface{}) {
	status, body, err := scimRequest(http.MethodPost, "/Users", ts.buildUserBody(email, extAttrs), nil)
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
// firstEmailValue handles first email value.
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

// TestCreateAndGetUser verifies a user can be created and then fetched via GET.
func (ts *SCIMUsersTestSuite) TestCreateAndGetUser() {
	email := "scim.it.create@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %v", created)

	gotEmail, ok := extensionStringValue(created, ts.extensionURN, "email")
	ts.Require().True(ok, "response should embed the extension object under its URN key")
	ts.Equal(email, gotEmail)
	ts.NotContains(created, "emails", "a non-core usertype must not surface core-mapped attributes")

	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var fetched map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &fetched))
	ts.Equal(id, fetched["id"])
	gotEmail, ok = extensionStringValue(fetched, ts.extensionURN, "email")
	ts.Require().True(ok, "GET response should include the custom-schema email attribute")
	ts.Equal(email, gotEmail)
}

// TestCreateCoreSchemaUserOnDesignatedType tests that the designated core user type accepts
// the core User schema and reverse-maps top-level emails into its own "email" property.
func (ts *SCIMUsersTestSuite) TestCreateCoreSchemaUserOnDesignatedType() {
	email := "scim.it.core-designated@example.com"
	body, err := json.Marshal(map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN, ts.coreExtensionURN},
		"userName": "scim.it.core-designated",
		"emails": []map[string]interface{}{
			{"value": email, "type": "work"},
		},
		ts.coreExtensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	ts.createdUserIDs = append(ts.createdUserIDs, id)

	gotEmail, ok := firstEmailValue(created)
	ts.Require().True(ok, "response should include the core-mapped emails field")
	ts.Equal(email, gotEmail)
}

// TestCreateCoreSchemaUserWithoutExtensionURN tests that a create carrying only the core User schema
// URN creates the user in the designated core user type.
func (ts *SCIMUsersTestSuite) TestCreateCoreSchemaUserWithoutExtensionURN() {
	id := ts.createCoreOnlyUser("scim.it.core-only-create", "scim.it.core-only-create@example.com")

	status, respBody, err := scimRequest(http.MethodGet, "/Users/"+id, nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "body: %s", respBody)

	var fetched map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &fetched))
	ts.Contains(fetched["schemas"], ts.coreExtensionURN, "the user should belong to the core user type")
}

// TestReplaceCoreSchemaUserWithoutExtensionURN tests that a replace carrying only the core User schema
// URN keeps the user's existing type.
func (ts *SCIMUsersTestSuite) TestReplaceCoreSchemaUserWithoutExtensionURN() {
	id := ts.createCoreOnlyUser("scim.it.core-only-replace", "scim.it.core-only-replace@example.com")

	email := "scim.it.core-only-replaced@example.com"
	body, err := json.Marshal(map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN},
		"userName": "scim.it.core-only-replace",
		"emails":   []map[string]interface{}{{"value": email, "type": "work"}},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPut, "/Users/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "replace failed: %s", respBody)

	var replaced map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &replaced))
	gotEmail, ok := firstEmailValue(replaced)
	ts.Require().True(ok, "response should include the core-mapped emails field")
	ts.Equal(email, gotEmail)
	ts.Contains(replaced["schemas"], ts.coreExtensionURN, "the user should keep the core user type")
}

// createCoreOnlyUser creates a user with only the core User schema URN and returns its ID.
func (ts *SCIMUsersTestSuite) createCoreOnlyUser(userName, email string) string {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN},
		"userName": userName,
		"emails":   []map[string]interface{}{{"value": email, "type": "work"}},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	ts.createdUserIDs = append(ts.createdUserIDs, id)
	return id
}

// TestCoreSchemaRejectedForNonDesignatedType tests that core User schema attributes sent to
// a user type other than the designated core user type are rejected.
func (ts *SCIMUsersTestSuite) TestCoreSchemaRejectedForNonDesignatedType() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":  []string{scimCoreUserSchemaURN, ts.extensionURN},
		"userName": "scim.it.core-rejected",
		"emails": []map[string]interface{}{
			{"value": "scim.it.core-rejected@example.com", "type": "work"},
		},
		ts.extensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %s", respBody)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(respBody, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
	ts.Contains(errResp.Detail, "designated core user type")
}

// TestListAndFilterByEmail verifies users can be listed and filtered by email.
func (ts *SCIMUsersTestSuite) TestListAndFilterByEmail() {
	email := "scim.it.filter@example.com"
	status, created := ts.createUser(email, nil)
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

// TestListUsersReturnsBareUserWithOrphanedType guards against a single user with an
// unresolvable entity type taking down an unfiltered, system-wide GET /Users list.
// DeleteUserType doesn't block deletion while users of that type still exist, so a
// user can end up pointing at a type that no longer exists - a reachable production
// state, not a synthetic one. The list must still return 200 and keep the user as a
// bare resource (id, core schema, meta, no attributes), so totalResults and paging
// stay consistent with the number of stored users.
func (ts *SCIMUsersTestSuite) TestListUsersReturnsBareUserWithOrphanedType() {
	// A known-good user that must still be listed once the orphan is present.
	status, goodUser := ts.createUser("scim.it.orphan-guard@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	goodUserID, _ := goodUser["id"].(string)

	orphanOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "scim-it-users-orphan-ou",
		Name:        "SCIM Users Orphaned-Type Test OU",
		Description: "OU whose user type gets deleted out from under its user",
	})
	ts.Require().NoError(err, "failed to create orphan-type test organization unit")
	defer func() { _ = testutils.DeleteOrganizationUnit(orphanOUID) }()

	orphanTypeName := "scim-it-users-orphan-person"
	orphanTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      orphanTypeName,
		DisplayName: "Orphan Type",
		OUID:        orphanOUID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "failed to create orphan-type test entity type")

	orphanURN, _, err := discoverExtensionSchema(orphanTypeName)
	ts.Require().NoError(err, "failed to discover orphan extension schema via GET /Schemas")

	payload := map[string]interface{}{
		"schemas": []string{orphanURN},
		orphanURN: map[string]interface{}{"email": "scim.it.users.orphan@example.com"},
	}
	body, err := json.Marshal(payload)
	ts.Require().NoError(err)
	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "failed to provision orphan-type fixture user: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	orphanUserID, _ := created["id"].(string)
	ts.Require().NotEmpty(orphanUserID)
	defer func() { _, _, _ = scimRequest(http.MethodDelete, "/Users/"+orphanUserID, nil, nil) }()

	// Orphan the user: delete its entity type while the user still exists.
	ts.Require().NoError(testutils.DeleteUserType(orphanTypeID), "failed to delete orphan user type")

	// Page through the full unfiltered list: the default GET /Users page size caps
	// well below the number of users a shared test OU can accumulate, so a
	// single-page fetch could miss goodUserID/orphanUserID simply because they
	// landed on a later page, not because the orphan-guard behavior is broken.
	usersByID := make(map[string]scimUser)
	for startIndex := 1; ; {
		path := fmt.Sprintf("/Users?startIndex=%d&count=%d", startIndex, scimPaginationMaxPageSize)
		status, listBody, err := scimRequest(http.MethodGet, path, nil, nil)
		ts.Require().NoError(err)
		ts.Require().Equal(http.StatusOK, status,
			"unfiltered GET /Users must not 500 due to one user's dangling type: %s", listBody)

		var list scimUserListResponse
		ts.Require().NoError(json.Unmarshal(listBody, &list))
		for _, u := range list.Resources {
			usersByID[u.ID] = u
		}
		if len(list.Resources) == 0 || len(usersByID) >= list.TotalResults {
			break
		}
		startIndex += len(list.Resources)
	}
	ts.Contains(usersByID, goodUserID, "other users must still be listed")
	orphan, ok := usersByID[orphanUserID]
	ts.Require().True(ok, "user with unresolvable type must still be listed")
	ts.Equal([]string{scimCoreUserSchemaURN}, orphan.Schemas,
		"user with unresolvable type must be returned bare, without an extension schema")
}

// TestReplaceUserUpdatesExtensionAttribute verifies PUT updates an extension attribute.
func (ts *SCIMUsersTestSuite) TestReplaceUserUpdatesExtensionAttribute() {
	email := "scim.it.replace@example.com"
	status, created := ts.createUser(email, map[string]interface{}{"department": "Support"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	// PUT is a full replace: the required core-mapped field must be resent.
	replaceBody := ts.buildUserBody(email, map[string]interface{}{"department": "Engineering"})
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
// missingRequiredAttrs fix: this usertype requires "email" — an extension
// object without it must fail with a 400 naming the missing attribute, not a
// generic schema-mismatch error.
func (ts *SCIMUsersTestSuite) TestMissingRequiredExtensionAttribute() {
	status, resp := ts.createUser("", nil)
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
	status, _ := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)

	status, resp := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusConflict, status, "expected 409, got body: %v", resp)

	body, err := json.Marshal(resp)
	ts.Require().NoError(err)
	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("uniqueness", errResp.ScimType)
}

// TestGetUnknownUserReturns404 verifies GET on an unknown user ID returns 404.
func (ts *SCIMUsersTestSuite) TestGetUnknownUserReturns404() {
	status, _, err := scimRequest(http.MethodGet, "/Users/does-not-exist", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status)
}

// TestDeleteUserThenGetReturns404 verifies a deleted user is gone: a subsequent GET returns 404.
func (ts *SCIMUsersTestSuite) TestDeleteUserThenGetReturns404() {
	status, created := ts.createUser("scim.it.delete@example.com", nil)
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

// TestCreateUserWrongContentTypeRejected verifies a create request with the wrong Content-Type is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserWrongContentTypeRejected() {
	body := ts.buildUserBody("scim.it.wrong-content-type@example.com", nil)
	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

// TestCreateUserMalformedJSONRejected verifies a create body that isn't valid JSON is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserMalformedJSONRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users", []byte(`{not valid json`), nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestCreateUserEmptyBodyRejected verifies an empty create body is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserEmptyBodyRejected() {
	status, _, err := scimRequest(http.MethodPost, "/Users", []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status)
}

// TestCreateUserConflictingAttributesQueryParamsRejected verifies attributes and
// excludedAttributes query params are mutually exclusive on create too, not just on GET.
func (ts *SCIMUsersTestSuite) TestCreateUserConflictingAttributesQueryParamsRejected() {
	body := ts.buildUserBody("scim.it.create-conflicting-attrs@example.com", nil)
	status, _, err := scimRequest(http.MethodPost,
		"/Users?attributes="+ts.extensionURN+":department&excludedAttributes="+ts.extensionURN+":team", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "attributes and excludedAttributes are mutually exclusive")
}

// TestReplaceUserConflictingAttributesQueryParamsRejected verifies attributes and
// excludedAttributes query params are mutually exclusive on replace too, not just on GET.
func (ts *SCIMUsersTestSuite) TestReplaceUserConflictingAttributesQueryParamsRejected() {
	email := "scim.it.replace-conflicting-attrs@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body := ts.buildUserBody(email, nil)
	status, _, err := scimRequest(http.MethodPut,
		"/Users/"+id+"?attributes="+ts.extensionURN+":department&excludedAttributes="+ts.extensionURN+":team", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "attributes and excludedAttributes are mutually exclusive")
}

// TestCreateUserMissingSchemasRejected verifies a request with no "schemas" array is rejected.
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

// TestCreateUserDuplicateSchemaURNsRejected verifies a duplicate URN in "schemas" is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserDuplicateSchemaURNsRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":       []string{scimCoreUserSchemaURN, ts.extensionURN, scimCoreUserSchemaURN},
		ts.extensionURN: map[string]interface{}{"email": "scim.it.duplicate-schema-urn@example.com"},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "duplicate URNs in \"schemas\" must be rejected")
}

// TestCreateUserTwoExtensionURNsRejected verifies more than one ThunderID extension URN is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserTwoExtensionURNsRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":          []string{ts.extensionURN, ts.altExtensionURN},
		ts.extensionURN:    map[string]interface{}{"email": "scim.it.two-extension-urns@example.com"},
		ts.altExtensionURN: map[string]interface{}{"email": "scim.it.two-extension-urns@example.com"},
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
	status, resp := ts.createUser("scim.it.undeclared-attr@example.com",
		map[string]interface{}{"nonexistent_field": "x"})
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)

	body, err := json.Marshal(resp)
	ts.Require().NoError(err)
	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
	ts.Contains(errResp.Detail, "nonexistent_field")
}

// TestCreateUserInvalidStringEnumRejected verifies a string value outside the declared enum is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserInvalidStringEnumRejected() {
	status, resp := ts.createUser("scim.it.invalid-string-enum@example.com",
		map[string]interface{}{"status": "not-a-declared-enum-value"})
	ts.Equal(http.StatusBadRequest, status, "expected 400, got body: %v", resp)
}

// TestCreateUserInvalidNumberEnumRejected verifies a number value outside the declared enum is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserInvalidNumberEnumRejected() {
	status, resp := ts.createUser("scim.it.invalid-number-enum@example.com",
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
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body, err := json.Marshal(map[string]interface{}{
		"schemas":          []string{ts.altExtensionURN},
		ts.altExtensionURN: map[string]interface{}{"email": email},
	})
	ts.Require().NoError(err)

	status, _, err = scimRequest(http.MethodPut, "/Users/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "replacing with a different usertype's extension URN must be rejected")
}

// TestReplaceUserIfMatchHeaderIgnored tests that a stale/bogus If-Match header is
// ignored rather than rejected, since ETag/version support is not implemented
// (ServiceProviderConfig advertises etag.supported=false).
func (ts *SCIMUsersTestSuite) TestReplaceUserIfMatchHeaderIgnored() {
	email := "scim.it.if-match@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	replaceBody := ts.buildUserBody(email, nil)
	status, _, err := scimRequest(http.MethodPut, "/Users/"+id, replaceBody,
		map[string]string{"If-Match": `"bogus-etag"`})
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, status)
}

// ---------------------------------------------------------------------------
// Attribute projection (RFC 7644 §3.9)
// ---------------------------------------------------------------------------

// TestGetUserAttributesProjection verifies attributes= projects the response down to the requested fields.
func (ts *SCIMUsersTestSuite) TestGetUserAttributesProjection() {
	email := "scim.it.projection.attrs@example.com"
	status, created := ts.createUser(email,
		map[string]interface{}{"department": "Engineering", "team": "Blue"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id+"?attributes="+ts.extensionURN+":department", nil, nil)
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

// TestGetUserExcludedAttributesProjection verifies excludedAttributes= drops the named fields from the response.
func (ts *SCIMUsersTestSuite) TestGetUserExcludedAttributesProjection() {
	email := "scim.it.projection.excluded@example.com"
	status, created := ts.createUser(email,
		map[string]interface{}{"department": "Engineering", "team": "Blue"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id+"?excludedAttributes="+ts.extensionURN+":team", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var projected map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &projected))

	ext, ok := projected[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok, "extension object should still be present")
	ts.NotContains(ext, "team", "excludedAttributes=team should drop team")
	ts.Contains(ext, "department", "excludedAttributes=team must keep the rest of the extension attributes")
}

// TestGetUserListAttributesProjection verifies attributes= projection also applies to list responses.
func (ts *SCIMUsersTestSuite) TestGetUserListAttributesProjection() {
	email := "scim.it.projection.list@example.com"
	status, created := ts.createUser(email, map[string]interface{}{"department": "Engineering"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	filter := url.QueryEscape(ts.extensionURN + `:email eq "` + email + `"`)
	status, body, err := scimRequest(
		http.MethodGet, "/Users?filter="+filter+"&attributes="+ts.extensionURN+":department", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &list))
	resources, ok := list["Resources"].([]interface{})
	ts.Require().True(ok)
	ts.Require().Len(resources, 1)
	resource, ok := resources[0].(map[string]interface{})
	ts.Require().True(ok)
	ts.Equal(id, resource["id"])
	ext, ok := resource[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok)
	ts.Contains(ext, "department")
}

// ---------------------------------------------------------------------------
// Additional request-shape and validation edge cases
// ---------------------------------------------------------------------------

// TestListUsersSortByRejected verifies sortBy/sortOrder is rejected when sorting is not supported.
func (ts *SCIMUsersTestSuite) TestListUsersSortByRejected() {
	status, _, err := scimRequest(http.MethodGet, "/Users?sortBy=userName", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "sortBy must be rejected when sorting is not supported")
}

// TestListUsersZeroCountReturnsNoResources verifies a negative count is clamped to 0 Resources.
func (ts *SCIMUsersTestSuite) TestListUsersZeroCountReturnsNoResources() {
	status, body, err := scimRequest(http.MethodGet, "/Users?count=-3", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var list scimUserListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))
	ts.Empty(list.Resources, "a negative count must be clamped to 0 Resources")
}

// TestConflictingAttributesParamsRejected verifies attributes and excludedAttributes are mutually exclusive.
func (ts *SCIMUsersTestSuite) TestConflictingAttributesParamsRejected() {
	status, _, err := scimRequest(http.MethodGet,
		"/Users?attributes="+ts.extensionURN+":department&excludedAttributes="+ts.extensionURN+":team", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "attributes and excludedAttributes are mutually exclusive")
}

// TestReplaceUserWrongContentTypeRejected verifies a replace request with the wrong Content-Type is rejected.
func (ts *SCIMUsersTestSuite) TestReplaceUserWrongContentTypeRejected() {
	email := "scim.it.replace-wrong-content-type@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	body := ts.buildUserBody(email, nil)
	status, respBody, err := scimRequest(http.MethodPut, "/Users/"+id, body, map[string]string{"Content-Type": "text/plain"})
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "wrong Content-Type must be rejected: %s", respBody)
}

// TestReplaceUserEmptyBodyRejected verifies an empty replace body is rejected.
func (ts *SCIMUsersTestSuite) TestReplaceUserEmptyBodyRejected() {
	email := "scim.it.replace-empty-body@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodPut, "/Users/"+id, []byte{}, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an empty body must be rejected")
}

// TestDeleteUnknownUserReturns404 verifies deleting an unknown user returns 404.
func (ts *SCIMUsersTestSuite) TestDeleteUnknownUserReturns404() {
	status, _, err := scimRequest(http.MethodDelete, "/Users/does-not-exist", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNotFound, status)
}

// TestReplaceUserUnknownUserTypeRejected verifies a replace naming a schema URN with no registered user type is rejected.
func (ts *SCIMUsersTestSuite) TestReplaceUserUnknownUserTypeRejected() {
	email := "scim.it.replace-unknown-type@example.com"
	status, created := ts.createUser(email, nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	// Derived from the discovered URN so it carries the configured custom-schema prefix and reaches the
	// user type lookup instead of being parsed as a core attribute.
	bogusURN := strings.Replace(ts.extensionURN, ts.entityTypeName, "scim-it-does-not-exist", 1)
	ts.Require().NotEqual(ts.extensionURN, bogusURN)
	body, err := json.Marshal(map[string]interface{}{
		"schemas": []string{bogusURN},
		bogusURN:  map[string]interface{}{"email": email},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPut, "/Users/"+id, body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a schema URN naming no registered user type must be rejected")
	ts.Contains(string(respBody), "does not exist", "the rejection must come from the unknown user type check")
}

// TestOptionsUsersPreflightAccepted verifies the CORS preflight handler is wired for the Users endpoint.
func (ts *SCIMUsersTestSuite) TestOptionsUsersPreflightAccepted() {
	status, _, err := scimRequest(http.MethodOptions, "/Users", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusNoContent, status)
}

// ---------------------------------------------------------------------------
// Schema body-shape edge cases (parseAndValidateSCIMUserRequest)
// ---------------------------------------------------------------------------

// TestCreateUserMultipleCustomSchemasInSchemasArrayRejected verifies more than one ThunderID extension URN in schemas is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserMultipleCustomSchemasInSchemasArrayRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":       []string{ts.extensionURN, ts.altExtensionURN},
		ts.extensionURN: map[string]interface{}{"email": "scim.it.multi-custom-schemas@example.com"},
	})
	ts.Require().NoError(err)

	status, _, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "more than one ThunderID extension URN in schemas must be rejected")
}

// TestCreateUserUndeclaredCustomSchemaObjectRejected verifies a thunderid-shaped body object not declared in schemas is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserUndeclaredCustomSchemaObjectRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":       []string{scimCoreUserSchemaURN},
		"userName":      "scim.it.undeclared-custom-object",
		ts.extensionURN: map[string]interface{}{"email": "scim.it.undeclared-custom-object@example.com"},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %s", respBody)
}

// TestCreateUserMissingCoreUserSchemaRejected verifies top-level core attributes without the core schema URN declared are rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserMissingCoreUserSchemaRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":           []string{ts.coreExtensionURN},
		"userName":          "scim.it.missing-core-schema",
		ts.coreExtensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status, "expected 400, got body: %s", respBody)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(respBody, &errResp))
	ts.Equal("invalidValue", errResp.ScimType)
}

// TestCreateUserUndeclaredEnterpriseSchemaObjectRejected verifies an enterprise object present without the enterprise schema declared is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserUndeclaredEnterpriseSchemaObjectRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":                   []string{ts.extensionURN},
		ts.extensionURN:             map[string]interface{}{"email": "scim.it.undeclared-enterprise-object@example.com"},
		scimEnterpriseUserSchemaURN: map[string]interface{}{"department": "x"},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status,
		"an enterprise object without the enterprise schema declared must be rejected: %s", respBody)
}

// TestCreateUserEnterpriseSchemaRejectedForNonDesignatedType verifies the enterprise schema, like the core schema, is only accepted on the designated user type.
func (ts *SCIMUsersTestSuite) TestCreateUserEnterpriseSchemaRejectedForNonDesignatedType() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":                   []string{ts.extensionURN, scimEnterpriseUserSchemaURN},
		ts.extensionURN:             map[string]interface{}{"email": "scim.it.enterprise-non-designated@example.com"},
		scimEnterpriseUserSchemaURN: map[string]interface{}{"department": "x"},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status,
		"the enterprise schema extension must be rejected for a non-designated-core user type: %s", respBody)
}

// TestCreateUserEnterpriseSchemaUndeclaredAttributeRejected verifies an enterprise attribute the designated type's own schema doesn't declare is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserEnterpriseSchemaUndeclaredAttributeRejected() {
	email := "scim.it.enterprise-undeclared-attr@example.com"
	body, err := json.Marshal(map[string]interface{}{
		"schemas":                   []string{scimCoreUserSchemaURN, ts.coreExtensionURN, scimEnterpriseUserSchemaURN},
		"userName":                  "scim.it.enterprise-undeclared-attr",
		"emails":                    []map[string]interface{}{{"value": email, "type": "work"}},
		ts.coreExtensionURN:         map[string]interface{}{},
		scimEnterpriseUserSchemaURN: map[string]interface{}{"department": "Engineering"},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status,
		"an enterprise attribute the designated type's schema does not declare must be rejected: %s", respBody)
}

// TestCreateUserConflictingCoreAndCustomValueRejected verifies a core-mapped attribute with different core and custom values is rejected.
func (ts *SCIMUsersTestSuite) TestCreateUserConflictingCoreAndCustomValueRejected() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":           []string{scimCoreUserSchemaURN, ts.coreExtensionURN},
		"userName":          "scim.it.conflict.core",
		"emails":            []map[string]interface{}{{"value": "scim.it.conflict@example.com", "type": "work"}},
		ts.coreExtensionURN: map[string]interface{}{"username": "scim.it.conflict.custom"},
	})
	ts.Require().NoError(err)

	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status,
		"a core-mapped attribute with conflicting core/custom values must be rejected: %s", respBody)

	var errResp2 scimErrorResponse
	ts.Require().NoError(json.Unmarshal(respBody, &errResp2))
	ts.Equal("invalidValue", errResp2.ScimType)
}

// ---------------------------------------------------------------------------
// attributes=/excludedAttributes= path validation (validateAttributePath)
// ---------------------------------------------------------------------------

// TestGetUserAttrsBareCustomRejected verifies a bare custom attribute name in attributes= requires a schema URN prefix.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsBareCustomRejected() {
	status, created := ts.createUser("scim.it.attrs.bare-custom@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(http.MethodGet, "/Users/"+id+"?attributes=department", nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, status)

	var errResp scimErrorResponse
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equal("invalidPath", errResp.ScimType)
}

// TestGetUserAttrsBareDottedRejected verifies a bare dotted sub-attribute path in attributes= is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsBareDottedRejected() {
	status, created := ts.createUser("scim.it.attrs.bare-dotted@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(http.MethodGet, "/Users/"+id+"?attributes=name.givenName", nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a dotted sub-attribute path is not supported for response projection")
}

// TestGetUserAttrsBadSchemaURNRejected verifies attributes= with an unrecognized schema URN prefix is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsBadSchemaURNRejected() {
	status, created := ts.createUser("scim.it.attrs.bad-urn@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	bogus := url.QueryEscape("urn:ietf:params:scim:schemas:extension:does-not-exist:2.0:User:field")
	status, _, err := scimRequest(http.MethodGet, "/Users/"+id+"?attributes="+bogus, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an unrecognized schema URN prefix must be rejected")
}

// TestGetUserAttrsCoreURNUnknownFieldRejected verifies attributes= with an unrecognized core-schema field is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsCoreURNUnknownFieldRejected() {
	status, created := ts.createUser("scim.it.attrs.core-unknown-field@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimCoreUserSchemaURN+":bogusField"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an unrecognized core-schema field must be rejected")
}

// TestGetUserAttrsCoreURNDottedRejected verifies a URN-qualified dotted core sub-attribute path in attributes= is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsCoreURNDottedRejected() {
	status, created := ts.createUser("scim.it.attrs.core-dotted@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimCoreUserSchemaURN+":name.givenName"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a URN-qualified dotted sub-attribute path must be rejected")
}

// TestGetUserAttrsEnterpriseURNUnknownFieldRejected verifies attributes= with an unrecognized enterprise-schema field is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsEnterpriseURNUnknownFieldRejected() {
	status, created := ts.createUser("scim.it.attrs.enterprise-unknown-field@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimEnterpriseUserSchemaURN+":bogusField"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an unrecognized enterprise-schema field must be rejected")
}

// TestGetUserAttrsEnterpriseURNDottedRejected verifies a URN-qualified dotted enterprise sub-attribute path in attributes= is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsEnterpriseURNDottedRejected() {
	status, created := ts.createUser("scim.it.attrs.enterprise-dotted@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimEnterpriseUserSchemaURN+":manager.value"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a URN-qualified dotted enterprise sub-attribute path must be rejected")
}

// TestGetUserAttrsExtURNFieldAccepted verifies a resolvable extension schema URN with any field name is accepted here; field existence isn't checked at this layer.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsExtURNFieldAccepted() {
	status, created := ts.createUser("scim.it.attrs.ext-field@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(ts.extensionURN+":bogusField"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, status, "a resolvable schema URN with any field name is accepted at this validation layer")
}

// TestGetUserAttrsExtURNDottedRejected verifies a URN-qualified dotted extension sub-attribute path in attributes= is rejected.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsExtURNDottedRejected() {
	status, created := ts.createUser("scim.it.attrs.ext-dotted@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(ts.extensionURN+":department.sub"), nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a URN-qualified dotted extension sub-attribute path must be rejected")
}

// TestGetUserAttrsWholeCoreURNKept verifies attributes=<core URN> keeps the whole core object as a unit.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsWholeCoreURNKept() {
	status, created := ts.createUser("scim.it.attrs.whole-core@example.com", nil)
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, _, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimCoreUserSchemaURN), nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)
}

// TestGetUserAttrsWholeEnterpriseURNKept verifies attributes=<enterprise URN> keeps the whole enterprise object as a unit.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsWholeEnterpriseURNKept() {
	body, err := json.Marshal(map[string]interface{}{
		"schemas":           []string{scimCoreUserSchemaURN, ts.coreExtensionURN},
		"userName":          "scim.it.attrs.whole-enterprise",
		"emails":            []map[string]interface{}{{"value": "scim.it.attrs.whole-enterprise@example.com", "type": "work"}},
		ts.coreExtensionURN: map[string]interface{}{},
	})
	ts.Require().NoError(err)
	status, respBody, err := scimRequest(http.MethodPost, "/Users", body, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, status, "expected 201, got body: %s", respBody)

	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(respBody, &created))
	id, _ := created["id"].(string)
	ts.Require().NotEmpty(id)
	ts.createdUserIDs = append(ts.createdUserIDs, id)

	status, _, err = scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(scimEnterpriseUserSchemaURN), nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)
}

// TestGetUserAttrsWholeExtURNKept verifies attributes=<extension URN> keeps the whole extension object as a unit.
func (ts *SCIMUsersTestSuite) TestGetUserAttrsWholeExtURNKept() {
	status, created := ts.createUser("scim.it.attrs.whole-ext@example.com", map[string]interface{}{"department": "Engineering"})
	ts.Require().Equal(http.StatusCreated, status)
	id, _ := created["id"].(string)

	status, body, err := scimRequest(
		http.MethodGet, "/Users/"+id+"?attributes="+url.QueryEscape(ts.extensionURN), nil, nil)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status)

	var projected map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &projected))
	ext, ok := projected[ts.extensionURN].(map[string]interface{})
	ts.Require().True(ok, "the whole extension object should be kept")
	ts.Contains(ext, "department")
}

// ---------------------------------------------------------------------------
// filter=<URN>:<field> schema-attribute validation (ValidateFilterSchemaAttribute)
// ---------------------------------------------------------------------------

// TestFilterCoreURNUnknownAttrRejected verifies a URN-qualified core filter attribute that doesn't exist is rejected.
func (ts *SCIMUsersTestSuite) TestFilterCoreURNUnknownAttrRejected() {
	filter := url.QueryEscape(scimCoreUserSchemaURN + `:bogusField eq "x"`)
	status, _, err := scimRequest(http.MethodGet, "/Users?filter="+filter, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an unrecognized URN-qualified core filter attribute must be rejected")
}

// TestFilterEnterpriseURNUnknownAttrRejected verifies a URN-qualified enterprise filter attribute that doesn't exist is rejected.
func (ts *SCIMUsersTestSuite) TestFilterEnterpriseURNUnknownAttrRejected() {
	filter := url.QueryEscape(scimEnterpriseUserSchemaURN + `:bogusField eq "x"`)
	status, _, err := scimRequest(http.MethodGet, "/Users?filter="+filter, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an unrecognized URN-qualified enterprise filter attribute must be rejected")
}

// TestFilterUnrecognizedSchemaURNRejected verifies a filter attribute with an unrecognized schema URN prefix is rejected.
func (ts *SCIMUsersTestSuite) TestFilterUnrecognizedSchemaURNRejected() {
	filter := url.QueryEscape("urn:ietf:params:scim:schemas:extension:does-not-exist:2.0:User:field eq \"x\"")
	status, _, err := scimRequest(http.MethodGet, "/Users?filter="+filter, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "a filter attribute with an unrecognized schema URN prefix must be rejected")
}

// TestFilterExtURNUnknownAttrRejected verifies a filter attribute the extension's own schema doesn't declare is rejected.
func (ts *SCIMUsersTestSuite) TestFilterExtURNUnknownAttrRejected() {
	filter := url.QueryEscape(ts.extensionURN + `:bogusField eq "x"`)
	status, _, err := scimRequest(http.MethodGet, "/Users?filter="+filter, nil, nil)
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, "an undeclared extension-schema filter attribute must be rejected")
}
