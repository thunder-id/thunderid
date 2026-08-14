// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package inboundclient covers the inbound-client validation layer. That package registers no HTTP
// routes of its own, so every branch here is driven through the application API, which is the
// surface that owns an inbound client's lifecycle.
package inboundclient

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

const applicationsPath = "/applications"

// InboundClientValidationSuite covers the foreign-key and user-attribute validation the inbound
// client applies when an application is created or updated: theme/layout references, allowed user
// types, subject attribute mapping, and the deliberate create-rejects / update-strips divergence
// for undeclared user attributes.
type InboundClientValidationSuite struct {
	suite.Suite
	client *http.Client
	ouID   string
	// userTypeName / userTypeID identify a user type declaring only the attributes below, so an
	// attribute outside this set is provably undeclared.
	userTypeName string
	userTypeID   string
	themeID      string
	layoutID     string
}

func TestInboundClientValidationSuite(t *testing.T) {
	suite.Run(t, new(InboundClientValidationSuite))
}

func (suite *InboundClientValidationSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Name:        "Inbound Client Test OU " + suffix,
		Handle:      "inbound-client-test-ou-" + suffix,
		Description: "OU for inbound client validation tests",
	})
	suite.Require().NoError(err, "Failed to create OU")
	suite.ouID = ouID

	suite.userTypeName = "inboundclient" + suffix
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: suite.userTypeName,
		OUID: suite.ouID,
		Schema: map[string]interface{}{
			"email": map[string]interface{}{
				"type":     "string",
				"unique":   true,
				"required": true,
			},
			"username": map[string]interface{}{
				"type": "string",
			},
		},
	})
	suite.Require().NoError(err, "Failed to create user type")
	suite.userTypeID = userTypeID

	suite.themeID = suite.createDesignResource("/design/themes", map[string]interface{}{
		"handle":      "inbound-client-theme-" + suffix,
		"displayName": "Inbound Client Theme",
		"theme":       map[string]interface{}{"direction": "ltr"},
	})
	suite.layoutID = suite.createDesignResource("/design/layouts", map[string]interface{}{
		"handle":      "inbound-client-layout-" + suffix,
		"displayName": "Inbound Client Layout",
		"layout":      map[string]interface{}{"type": "centered"},
	})
}

func (suite *InboundClientValidationSuite) TearDownSuite() {
	suite.deleteDesignResource("/design/themes", suite.themeID)
	suite.deleteDesignResource("/design/layouts", suite.layoutID)

	if suite.userTypeID != "" {
		if err := testutils.DeleteUserType(suite.userTypeID); err != nil {
			suite.T().Logf("Failed to delete user type %s: %v", suite.userTypeID, err)
		}
	}
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete OU %s: %v", suite.ouID, err)
		}
	}
}

// --- Theme / layout foreign keys ---

func (suite *InboundClientValidationSuite) TestCreateWithUnknownLayoutIsRejected() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":     suite.appName("unknown-layout"),
		"layoutId": "00000000-0000-0000-0000-000000000000",
	})

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("APP-1027", suite.errorCode(body))
}

func (suite *InboundClientValidationSuite) TestCreateWithValidThemeAndLayoutSucceeds() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":     suite.appName("valid-design"),
		"themeId":  suite.themeID,
		"layoutId": suite.layoutID,
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	appID := suite.idOf(body)
	defer suite.deleteApplication(appID)

	fetched := suite.getApplication(appID)
	suite.Equal(suite.themeID, fetched["themeId"])
	suite.Equal(suite.layoutID, fetched["layoutId"])
}

// TestUpdateToUnknownLayoutIsRejected asserts the FK check runs on update, not only on create.
func (suite *InboundClientValidationSuite) TestUpdateToUnknownLayoutIsRejected() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":     suite.appName("update-unknown-layout"),
		"layoutId": suite.layoutID,
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	appID := suite.idOf(body)
	defer suite.deleteApplication(appID)

	updateStatus, updateBody := suite.updateApplication(appID, map[string]interface{}{
		"name":     suite.appName("update-unknown-layout"),
		"layoutId": "00000000-0000-0000-0000-000000000000",
	})

	suite.Equal(http.StatusBadRequest, updateStatus)
	suite.Equal("APP-1027", suite.errorCode(updateBody))
}

// TestDeletingReferencedLayoutThenUpdatingIsRejected pins that a dangling reference cannot be
// re-saved: layouts may be deleted while referenced, but a later update revalidates the FK.
func (suite *InboundClientValidationSuite) TestDeletingReferencedLayoutThenUpdatingIsRejected() {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	throwawayLayoutID := suite.createDesignResource("/design/layouts", map[string]interface{}{
		"handle":      "inbound-client-throwaway-" + suffix,
		"displayName": "Throwaway Layout",
		"layout":      map[string]interface{}{"type": "centered"},
	})

	status, body := suite.createApplication(map[string]interface{}{
		"name":     suite.appName("dangling-layout"),
		"layoutId": throwawayLayoutID,
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	appID := suite.idOf(body)
	defer suite.deleteApplication(appID)

	suite.deleteDesignResource("/design/layouts", throwawayLayoutID)

	updateStatus, updateBody := suite.updateApplication(appID, map[string]interface{}{
		"name":     suite.appName("dangling-layout"),
		"layoutId": throwawayLayoutID,
	})

	suite.Equal(http.StatusBadRequest, updateStatus)
	suite.Equal("APP-1027", suite.errorCode(updateBody))
}

// Allowed user type validation (APP-1025) is covered by the application suite, on create, on
// update, and for a mixed valid/invalid list; it is deliberately not repeated here.

// --- User attributes: create rejects, update strips ---

// TestCreateWithUndeclaredUserAttributeIsRejected pins the create-side behaviour: an assertion
// attribute no allowed user type declares fails the request.
func (suite *InboundClientValidationSuite) TestCreateWithUndeclaredUserAttributeIsRejected() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":             suite.appName("undeclared-attr-create"),
		"allowedUserTypes": []string{suite.userTypeName},
		"assertion": map[string]interface{}{
			"userAttributes": []string{"email", "not_a_declared_attribute"},
		},
	})

	suite.Equal(http.StatusBadRequest, status)
	suite.Equal("APP-1035", suite.errorCode(body))
}

// TestUpdateWithUndeclaredUserAttributeStripsIt is the deliberate divergence from create: an
// update silently drops undeclared attributes instead of rejecting the request.
func (suite *InboundClientValidationSuite) TestUpdateWithUndeclaredUserAttributeStripsIt() {
	name := suite.appName("undeclared-attr-update")
	status, body := suite.createApplication(map[string]interface{}{
		"name":             name,
		"allowedUserTypes": []string{suite.userTypeName},
		"assertion": map[string]interface{}{
			"userAttributes": []string{"email"},
		},
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	appID := suite.idOf(body)
	defer suite.deleteApplication(appID)

	updateStatus, updateBody := suite.updateApplication(appID, map[string]interface{}{
		"name":             name,
		"allowedUserTypes": []string{suite.userTypeName},
		"assertion": map[string]interface{}{
			"userAttributes": []string{"email", "not_a_declared_attribute"},
		},
	})
	suite.Require().Equal(http.StatusOK, updateStatus, "body: %s", updateBody)

	fetched := suite.getApplication(appID)
	attrs := suite.assertionUserAttributes(fetched)
	suite.Contains(attrs, "email")
	suite.NotContains(attrs, "not_a_declared_attribute",
		"an update must strip undeclared attributes rather than persist them")
}

// TestCreateWithDeclaredUserAttributesSucceeds is the control for the two tests above.
func (suite *InboundClientValidationSuite) TestCreateWithDeclaredUserAttributesSucceeds() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":             suite.appName("declared-attrs"),
		"allowedUserTypes": []string{suite.userTypeName},
		"assertion": map[string]interface{}{
			"userAttributes": []string{"email", "username"},
		},
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	appID := suite.idOf(body)
	defer suite.deleteApplication(appID)

	attrs := suite.assertionUserAttributes(suite.getApplication(appID))
	suite.Contains(attrs, "email")
	suite.Contains(attrs, "username")
}

// --- Subject attribute mapping ---

// TestCreateWithValidSubjectAttributeSucceeds is the control: "email" is unique and required.
func (suite *InboundClientValidationSuite) TestCreateWithValidSubjectAttributeSucceeds() {
	status, body := suite.createApplication(map[string]interface{}{
		"name":             suite.appName("valid-subject-attr"),
		"allowedUserTypes": []string{suite.userTypeName},
		"subjectAttribute": map[string]string{suite.userTypeName: "email"},
	})
	suite.Require().Equal(http.StatusCreated, status, "body: %s", body)

	suite.deleteApplication(suite.idOf(body))
}

// --- helpers ---

// appName builds a name unique to this suite run so parallel packages do not collide.
func (suite *InboundClientValidationSuite) appName(label string) string {
	return fmt.Sprintf("Inbound Client %s %d", label, time.Now().UnixNano())
}

func (suite *InboundClientValidationSuite) createApplication(body map[string]interface{}) (int, []byte) {
	return suite.do(http.MethodPost, testutils.TestServerURL+applicationsPath, suite.withRequiredFields(body))
}

func (suite *InboundClientValidationSuite) updateApplication(
	id string, body map[string]interface{},
) (int, []byte) {
	return suite.do(http.MethodPut, testutils.TestServerURL+applicationsPath+"/"+id,
		suite.withRequiredFields(body))
}

// withRequiredFields fills in ouId and type, which the application API requires on every request,
// so each test body only carries the fields whose validation it exercises.
func (suite *InboundClientValidationSuite) withRequiredFields(
	body map[string]interface{},
) map[string]interface{} {
	if _, ok := body["ouId"]; !ok {
		body["ouId"] = suite.ouID
	}
	if _, ok := body["type"]; !ok {
		body["type"] = "fullstack"
	}
	return body
}

func (suite *InboundClientValidationSuite) getApplication(id string) map[string]interface{} {
	status, body := suite.do(http.MethodGet, testutils.TestServerURL+applicationsPath+"/"+id, nil)
	suite.Require().Equal(http.StatusOK, status, "get application failed: %s", body)

	var app map[string]interface{}
	suite.Require().NoError(json.Unmarshal(body, &app))
	return app
}

func (suite *InboundClientValidationSuite) deleteApplication(id string) {
	if id == "" {
		return
	}
	if err := testutils.DeleteApplication(id); err != nil {
		suite.T().Logf("Failed to delete application %s: %v", id, err)
	}
}

// assertionUserAttributes reads assertion.userAttributes off an application response.
func (suite *InboundClientValidationSuite) assertionUserAttributes(app map[string]interface{}) []string {
	assertion, ok := app["assertion"].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := assertion["userAttributes"].([]interface{})
	if !ok {
		return nil
	}

	attrs := make([]string, 0, len(raw))
	for _, item := range raw {
		if str, ok := item.(string); ok {
			attrs = append(attrs, str)
		}
	}
	return attrs
}

// createDesignResource creates a theme or layout and returns its ID.
func (suite *InboundClientValidationSuite) createDesignResource(
	basePath string, body map[string]interface{},
) string {
	status, respBody := suite.do(http.MethodPost, testutils.TestServerURL+basePath, body)
	suite.Require().Equal(http.StatusCreated, status, "create %s failed: %s", basePath, respBody)
	return suite.idOf(respBody)
}

// deleteDesignResource removes a theme or layout, tolerating an already-deleted resource.
func (suite *InboundClientValidationSuite) deleteDesignResource(basePath, id string) {
	if id == "" {
		return
	}

	status, body := suite.do(http.MethodDelete, testutils.TestServerURL+basePath+"/"+id, nil)
	if status != http.StatusNoContent && status != http.StatusNotFound {
		suite.T().Logf("Failed to delete %s/%s: status %d: %s", basePath, id, status, body)
	}
}

// idOf extracts the id field from a created-resource response body.
func (suite *InboundClientValidationSuite) idOf(body []byte) string {
	var created struct {
		ID string `json:"id"`
	}
	suite.Require().NoError(json.Unmarshal(body, &created), "body: %s", body)
	suite.Require().NotEmpty(created.ID)
	return created.ID
}

// do issues a request with an optional JSON body and returns the status and response body.
func (suite *InboundClientValidationSuite) do(
	method, target string, body map[string]interface{},
) (int, []byte) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		suite.Require().NoError(err)
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, target, reader)
	suite.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			suite.T().Logf("Failed to close response body: %v", closeErr)
		}
	}()

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)
	return resp.StatusCode, respBody
}

// errorCode decodes the error code from an API error response body.
func (suite *InboundClientValidationSuite) errorCode(body []byte) string {
	var errResp struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
