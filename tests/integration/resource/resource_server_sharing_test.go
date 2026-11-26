// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The resource server API answers a sharing refusal in its own vocabulary.
const (
	errInvalidSharingPolicy         = "RES-1029"
	errSharingPolicyNotFound        = "RES-1030"
	errSharingPolicyExists          = "RES-1031"
	errSharingPolicyVersionMismatch = "RES-1032"
	errResourceServerNotSharedToOU  = "RES-1034"
	errMissingOUIDParameter         = "RES-1035"
	errDeploymentWideSharing        = "RES-1036"
	errSharedResourceServerMove     = "RES-1037"
	errPermissionsRuleNotSupported  = "RES-1038"
	errResourceServerNotFound       = "RES-1003"
)

// ResourceServerSharingTestSuite covers sharing a resource server with other organization units through its API.
type ResourceServerSharingTestSuite struct {
	suite.Suite
	ownerOUID      string
	childOUID      string
	grandchildOUID string
	otherOUID      string
}

func TestResourceServerSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingTestSuite))
}

func (s *ResourceServerSharingTestSuite) SetupSuite() {
	s.ownerOUID = s.createOU("rs-sharing-owner", nil)
	s.childOUID = s.createOU("rs-sharing-child", &s.ownerOUID)
	s.grandchildOUID = s.createOU("rs-sharing-grandchild", &s.childOUID)
	s.otherOUID = s.createOU("rs-sharing-other", nil)
}

func (s *ResourceServerSharingTestSuite) TearDownSuite() {
	for _, id := range []string{s.grandchildOUID, s.childOUID, s.ownerOUID, s.otherOUID} {
		if id != "" {
			_ = testutils.DeleteOrganizationUnit(id)
		}
	}
}

func (s *ResourceServerSharingTestSuite) createOU(handle string, parent *string) string {
	s.T().Helper()
	id, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle, Parent: parent})
	s.Require().NoError(err)
	return id
}

// createServer creates a resource server owned by the suite's owner and removes it when the test ends.
func (s *ResourceServerSharingTestSuite) createServer(name string) string {
	s.T().Helper()
	id, err := createResourceServer(CreateResourceServerRequest{Name: name, OUID: s.ownerOUID})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteResourceServer(id) })
	return id
}

// call sends a sharing request and returns the status and the decoded body.
func (s *ResourceServerSharingTestSuite) call(method, url string, body interface{}) (int, map[string]interface{}) {
	s.T().Helper()
	var res *rawResponse
	var err error
	if body == nil {
		res, err = doRawRequest(method, url, nil)
	} else {
		res, err = doJSONRequest(method, url, body)
	}
	s.Require().NoError(err)
	decoded := map[string]interface{}{}
	if res.Body != "" {
		_ = json.Unmarshal([]byte(res.Body), &decoded)
	}
	return res.StatusCode, decoded
}

func policiesURL(serverID string) string {
	return resourceServerURL("/%s/sharing-policies", serverID)
}

func overlayURL(serverID, ouID string) string {
	return resourceServerURL("/%s/overlay-rules?ouId=%s", serverID, ouID)
}

// permissionsRule is a policy reaching every child unit with one overlay rule on fieldKey.
func permissionsRule(fieldKey string, rule map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"childUnits": map[string]interface{}{
		"mode": "all", "overlayRules": map[string]interface{}{fieldKey: rule},
	}}
}

func allChildUnits(excluded ...string) map[string]interface{} {
	section := map[string]interface{}{"mode": "all"}
	if len(excluded) > 0 {
		section["excludedOuIds"] = excluded
	}
	return map[string]interface{}{"childUnits": section}
}

// A policy is created, read, edited and deleted through the resource server it shares, and the organization unit it
// reaches can be resolved at each step.
func (s *ResourceServerSharingTestSuite) TestSharingPolicyLifecycle() {
	serverID := s.createServer("Sharing Lifecycle API")

	status, body := s.call(http.MethodGet, overlayURL(serverID, s.childOUID), nil)
	s.Require().Equal(http.StatusNotFound, status, "body: %v", body)
	s.Equal(errResourceServerNotSharedToOU, body["code"], "nothing reaches the child before a policy does")

	status, policy := s.call(http.MethodPost, policiesURL(serverID), allChildUnits())
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)
	s.Require().NotEmpty(policyID)
	s.Equal("share", policy["stage"])
	s.Equal(s.ownerOUID, policy["initiatingOuId"])
	s.Equal(float64(1), policy["version"])

	status, overlay := s.call(http.MethodGet, overlayURL(serverID, s.grandchildOUID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
	s.Equal(true, overlay["visible"], "every child unit reaches the grandchild too")
	s.Equal(false, overlay["isOwnedByOU"])
	s.Equal(map[string]interface{}{"permissions": map[string]interface{}{
		"editable": false, "value": []interface{}{}}}, overlay["rules"],
		"the whole server is shared, and this one defines no permission yet")

	status, owner := s.call(http.MethodGet, overlayURL(serverID, s.ownerOUID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", owner)
	s.Equal(true, owner["isOwnedByOU"])

	status, fetched := s.call(http.MethodGet, policiesURL(serverID)+"/"+policyID, nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", fetched)
	childUnits, _ := fetched["childUnits"].(map[string]interface{})
	s.Require().NotNil(childUnits)
	s.Equal("all", childUnits["mode"])

	status, listed := s.call(http.MethodGet, policiesURL(serverID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	s.Equal(float64(1), listed["totalResults"])

	// Excluding the child withdraws it, and the grandchild beneath it.
	update := allChildUnits(s.childOUID)
	update["version"] = 1
	status, edited := s.call(http.MethodPut, policiesURL(serverID)+"/"+policyID, update)
	s.Require().Equal(http.StatusOK, status, "body: %v", edited)
	s.Equal(float64(2), edited["version"])
	status, body = s.call(http.MethodGet, overlayURL(serverID, s.grandchildOUID), nil)
	s.Equal(http.StatusNotFound, status, "body: %v", body)

	status, body = s.call(http.MethodPut, policiesURL(serverID)+"/"+policyID, update)
	s.Require().Equal(http.StatusConflict, status, "body: %v", body)
	s.Equal(errSharingPolicyVersionMismatch, body["code"])

	status, _ = s.call(http.MethodDelete, policiesURL(serverID)+"/"+policyID, nil)
	s.Equal(http.StatusNoContent, status)
	status, _ = s.call(http.MethodDelete, policiesURL(serverID)+"/"+policyID, nil)
	s.Equal(http.StatusNoContent, status, "deleting a policy that is gone succeeds")
	status, body = s.call(http.MethodGet, policiesURL(serverID)+"/"+policyID, nil)
	s.Require().Equal(http.StatusNotFound, status, "body: %v", body)
	s.Equal(errSharingPolicyNotFound, body["code"])
}

// An organization unit the resource server reaches can share it onward to the units beneath itself.
func (s *ResourceServerSharingTestSuite) TestReshare() {
	serverID := s.createServer("Reshare API")
	status, body := s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"childUnits": map[string]interface{}{"mode": "selected", "units": []map[string]interface{}{
			{"ouId": s.childOUID, "reach": "unit"},
		}},
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)

	status, body = s.call(http.MethodGet, overlayURL(serverID, s.grandchildOUID), nil)
	s.Require().Equal(http.StatusNotFound, status, "a reach of unit stops at the child: %v", body)

	status, reshare := s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"initiatingOuId": s.childOUID,
		"childUnits":     map[string]interface{}{"mode": "all"},
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", reshare)
	s.Equal("reshare", reshare["stage"])

	status, overlay := s.call(http.MethodGet, overlayURL(serverID, s.grandchildOUID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
	s.Equal(true, overlay["visible"])

	status, listed := s.call(http.MethodGet, policiesURL(serverID), nil)
	s.Require().Equal(http.StatusOK, status)
	s.Equal(float64(2), listed["totalResults"])
}

func (s *ResourceServerSharingTestSuite) TestRefusedRequests() {
	serverID := s.createServer("Refusals API")
	otherServerID := s.createServer("Refusals Other API")
	status, created := s.call(http.MethodPost, policiesURL(otherServerID), allChildUnits())
	s.Require().Equal(http.StatusCreated, status, "body: %v", created)
	foreignPolicyID, _ := created["id"].(string)

	cases := []struct {
		name       string
		method     string
		url        string
		body       interface{}
		wantStatus int
		wantCode   string
	}{
		{name: "a deployment-wide reach", method: http.MethodPost, url: policiesURL(serverID),
			body:       map[string]interface{}{"allUnits": map[string]interface{}{}},
			wantStatus: http.StatusBadRequest, wantCode: errDeploymentWideSharing},
		{name: "an overlay rule on a field a resource server does not expose", method: http.MethodPost,
			url: policiesURL(serverID), body: permissionsRule("roles", map[string]interface{}{"editable": false}),
			wantStatus: http.StatusBadRequest, wantCode: errInvalidSharingPolicy},
		{name: "a permissions rule naming what it shares", method: http.MethodPost, url: policiesURL(serverID),
			body: permissionsRule("permissions", map[string]interface{}{
				"editable": false, "value": []string{"bookings:read"}}),
			wantStatus: http.StatusBadRequest, wantCode: errPermissionsRuleNotSupported},
		{name: "a permissions rule naming what it withholds", method: http.MethodPost, url: policiesURL(serverID),
			body: permissionsRule("permissions", map[string]interface{}{
				"editable": false, "excludedValues": []string{"bookings:delete"}}),
			wantStatus: http.StatusBadRequest, wantCode: errPermissionsRuleNotSupported},
		{name: "a permissions rule bounding a choice", method: http.MethodPost, url: policiesURL(serverID),
			body: permissionsRule("permissions", map[string]interface{}{
				"editable": false, "allowedValues": []string{"bookings"}}),
			wantStatus: http.StatusBadRequest, wantCode: errInvalidSharingPolicy},
		{name: "an editable permissions rule", method: http.MethodPost, url: policiesURL(serverID),
			body:       permissionsRule("permissions", map[string]interface{}{"editable": true}),
			wantStatus: http.StatusBadRequest, wantCode: errInvalidSharingPolicy},
		{name: "a policy reaching nothing", method: http.MethodPost, url: policiesURL(serverID),
			body: map[string]interface{}{}, wantStatus: http.StatusBadRequest, wantCode: errInvalidSharingPolicy},
		{name: "another resource server's policy", method: http.MethodGet,
			url: policiesURL(serverID) + "/" + foreignPolicyID, wantStatus: http.StatusNotFound,
			wantCode: errSharingPolicyNotFound},
		{name: "an overlay with no organization unit", method: http.MethodGet,
			url: resourceServerURL("/%s/overlay-rules", serverID), wantStatus: http.StatusBadRequest,
			wantCode: errMissingOUIDParameter},
		{name: "an unknown resource server", method: http.MethodPost,
			url: policiesURL("00000000-0000-7000-8000-000000000000"), body: allChildUnits(),
			wantStatus: http.StatusNotFound, wantCode: errResourceServerNotFound},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			status, body := s.call(tc.method, tc.url, tc.body)
			s.Equal(tc.wantStatus, status, "body: %v", body)
			s.Equal(tc.wantCode, body["code"])
		})
	}
}

// A reached organization unit holds every permission the resource server defines, fixed, and sees them listed:
// the server's own actions, its resources at every level, and their actions.
func (s *ResourceServerSharingTestSuite) TestOverlayListsEveryPermission() {
	delimiter := ":"
	serverID, err := createResourceServer(CreateResourceServerRequest{
		Name: "Permissions Overlay API", OUID: s.ownerOUID, Delimiter: &delimiter,
	})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteResourceServer(serverID) })
	// Cleanups run last first, so the actions go before the resources and the resources before the server.
	orders, err := createResource(serverID, CreateResourceRequest{Name: "Orders", Handle: "orders"})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteResource(serverID, orders) })
	items, err := createResource(serverID, CreateResourceRequest{Name: "Items", Handle: "items", Parent: &orders})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteResource(serverID, items) })
	read, err := createActionAtResource(serverID, orders, CreateActionRequest{Name: "Read", Handle: "read"})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteActionAtResource(serverID, orders, read) })
	write, err := createActionAtResource(serverID, items, CreateActionRequest{Name: "Write", Handle: "write"})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteActionAtResource(serverID, items, write) })
	admin, err := createActionAtResourceServer(serverID, CreateActionRequest{Name: "Admin", Handle: "admin"})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = deleteAction(serverID, admin) })

	status, policy := s.call(http.MethodPost, policiesURL(serverID),
		permissionsRule("permissions", map[string]interface{}{"editable": false}))
	s.Require().Equal(http.StatusCreated, status, "a fixed rule naming no value shares the whole server: %v", policy)

	every := map[string]interface{}{"permissions": map[string]interface{}{"editable": false, "value": []interface{}{
		"admin", "orders", "orders:items", "orders:items:write", "orders:read"}}}
	for _, ouID := range []string{s.childOUID, s.ownerOUID} {
		status, overlay := s.call(http.MethodGet, overlayURL(serverID, ouID), nil)
		s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
		s.Equal(every, overlay["rules"], "%s holds every permission", ouID)
	}
}

// A list filtered by organization unit holds the resource servers the unit owns, then the ones shared with it, which
// are listed under the unit and read only there.
func (s *ResourceServerSharingTestSuite) TestOUFilteredListIncludesSharedServers() {
	serverID := s.createServer("Listed Shared API")
	status, body := s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]interface{}{{"ouId": s.childOUID, "reach": "unit"}},
		},
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)

	find := func(ouID string) map[string]interface{} {
		s.T().Helper()
		status, list := s.call(http.MethodGet,
			resourceServerURL("?limit=100&filter=%s", url.QueryEscape(`ouId eq "`+ouID+`"`)), nil)
		s.Require().Equal(http.StatusOK, status, "body: %v", list)
		items, _ := list["resourceServers"].([]interface{})
		for _, item := range items {
			server := item.(map[string]interface{})
			if server["id"] == serverID {
				return server
			}
		}
		return nil
	}

	owned := find(s.ownerOUID)
	s.Require().NotNil(owned, "the owner lists its resource server")
	s.Equal(true, owned["isOwnedByOU"])

	shared := find(s.childOUID)
	s.Require().NotNil(shared, "the child lists the resource server shared with it")
	s.Equal(false, shared["isOwnedByOU"])
	s.Equal(true, shared["isReadOnly"])
	s.Equal(s.childOUID, shared["ouId"], "a shared resource server is listed under the unit it is shared with")

	s.Nil(find(s.otherOUID), "a unit the resource server is not shared with does not list it")
}

func (s *ResourceServerSharingTestSuite) TestSecondPolicyOfTheSameUnitIsRefused() {
	serverID := s.createServer("Duplicate Policy API")
	status, body := s.call(http.MethodPost, policiesURL(serverID), allChildUnits())
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)

	status, body = s.call(http.MethodPost, policiesURL(serverID), allChildUnits())
	s.Equal(http.StatusConflict, status, "body: %v", body)
	s.Equal(errSharingPolicyExists, body["code"])
}

// A shared resource server stays in its organization unit until its policies are deleted.
func (s *ResourceServerSharingTestSuite) TestSharedResourceServerCannotMove() {
	serverID := s.createServer("Move API")
	status, policy := s.call(http.MethodPost, policiesURL(serverID), allChildUnits())
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)

	move := UpdateResourceServerRequest{Name: "Move API", OUID: s.otherOUID}
	status, body := s.call(http.MethodPut, resourceServerURL("/%s", serverID), move)
	s.Require().Equal(http.StatusBadRequest, status, "body: %v", body)
	s.Equal(errSharedResourceServerMove, body["code"])

	status, _ = s.call(http.MethodDelete, policiesURL(serverID)+"/"+policyID, nil)
	s.Require().Equal(http.StatusNoContent, status)
	status, body = s.call(http.MethodPut, resourceServerURL("/%s", serverID), move)
	s.Equal(http.StatusOK, status, "an unshared resource server moves: %v", body)
}

// Deleting a resource server deletes its policies, so the organization units they reached lose it.
func (s *ResourceServerSharingTestSuite) TestDeleteResourceServerRemovesItsPolicies() {
	id, err := createResourceServer(CreateResourceServerRequest{Name: "Deleted Shared API", OUID: s.ownerOUID})
	s.Require().NoError(err)
	status, body := s.call(http.MethodPost, policiesURL(id), allChildUnits())
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)

	s.Require().NoError(deleteResourceServer(id))

	status, body = s.call(http.MethodGet, policiesURL(id), nil)
	s.Equal(http.StatusNotFound, status, "body: %v", body)
	s.Equal(errResourceServerNotFound, body["code"])
}

// A resource server file declares its owner's policy, which the API lists as read only and refuses to change, while
// an organization unit it reaches can still reshare it through the API.
func (s *ResourceServerSharingTestSuite) TestDeclaredPolicy() {
	const (
		serverID        = "decl-rs-shared"
		declaredPolicy  = "decl-rs-shared-policy-owner"
		declaredOwner   = "decl-m2m-root"
		declaredChildA  = "decl-m2m-child-a"
		declaredChildB  = "decl-m2m-child-b"
		declaredGrandA  = "decl-m2m-grandchild-a"
		immutableServer = "RES-1018"
	)

	status, listed := s.call(http.MethodGet, policiesURL(serverID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", listed)
	s.Require().Equal(float64(1), listed["totalResults"])
	policy := listed["policies"].([]interface{})[0].(map[string]interface{})
	s.Equal(declaredPolicy, policy["id"])
	s.Equal(declaredOwner, policy["initiatingOuId"], "the owner named by handle is resolved to its id")
	s.Equal(true, policy["isReadOnly"])

	status, overlay := s.call(http.MethodGet, overlayURL(serverID, declaredChildA), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
	s.Equal(true, overlay["visible"])
	s.Equal(map[string]interface{}{"permissions": map[string]interface{}{
		"editable": false, "value": []interface{}{"orders", "orders:read"}}}, overlay["rules"])
	for _, ouID := range []string{declaredChildB, declaredGrandA} {
		status, body := s.call(http.MethodGet, overlayURL(serverID, ouID), nil)
		s.Equal(http.StatusNotFound, status, "%s is not reached: %v", ouID, body)
	}

	status, body := s.call(http.MethodPost, policiesURL(serverID), allChildUnits())
	s.Equal(http.StatusBadRequest, status, "body: %v", body)
	s.Equal(immutableServer, body["code"], "the owner's sharing of a declared resource server is declared")

	status, body = s.call(http.MethodDelete, policiesURL(serverID)+"/"+declaredPolicy, nil)
	s.Equal(http.StatusBadRequest, status, "body: %v", body)
	s.Equal(immutableServer, body["code"])

	status, reshare := s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"initiatingOuId": declaredChildA,
		"childUnits":     map[string]interface{}{"mode": "all"},
	})
	s.Require().Equal(http.StatusCreated, status, "a reached unit reshares a declared resource server: %v", reshare)
	reshareID, _ := reshare["id"].(string)
	defer func() { _, _ = s.call(http.MethodDelete, policiesURL(serverID)+"/"+reshareID, nil) }()

	status, overlay = s.call(http.MethodGet, overlayURL(serverID, declaredGrandA), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
	s.Equal(true, overlay["visible"])
}

// adminToken returns a token that holds org in the organization unit: an application there, named after name, with a
// role assigned to it there. The System resource server is shared with the tree of systemRoot first, so the role can
// hold org.
func (s *ResourceServerSharingTestSuite) adminToken(ouID, systemRoot, name string) string {
	s.T().Helper()
	unshareSystem, err := testutils.ShareSystemResourceServerWith(systemRoot)
	s.Require().NoError(err, "share the System resource server")
	s.T().Cleanup(func() { s.NoError(unshareSystem()) })

	clientID, clientSecret := name, name+"-secret"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: name,
		OUID: ouID,
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                clientID,
				"clientSecret":            clientSecret,
				"grantTypes":              []string{"client_credentials"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				// An org permission acts in the unit the token names.
				"token": map[string]interface{}{"accessToken": map[string]interface{}{
					"clientConfig": map[string]interface{}{"attributes": []string{"ouId"}},
				}},
			},
		}},
	})
	s.Require().NoError(err, "create the admin application")
	s.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: name, OUID: ouID,
		Permissions: []testutils.ResourcePermissions{{
			ResourceServerID: "01900000-0000-7000-8000-000000000020", Permissions: []string{"org"},
		}},
		Assignments: []testutils.Assignment{{ID: appID, Type: "app"}},
	})
	s.Require().NoError(err, "create the admin role")
	s.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })

	token, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "org",
		testutils.SystemResourceIdentifier)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, token.StatusCode, "token: %s", string(token.Body))
	s.Require().Equal("org", token.Token.Scope)
	return token.Token.AccessToken
}

// An administrator confined to an organization unit reads a resource server another tree shares with that unit, with
// its resources and actions, as the unit sees it: listed under the unit and read only. It cannot change it, and
// cannot read a resource server of that tree that is not shared with it.
func (s *ResourceServerSharingTestSuite) TestSharedServerIsReadableInTheUnitItIsSharedWith() {
	serverID := s.createServer("Read In Other Tree API")
	s.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(serverID) })
	resourceID, err := createResource(serverID, CreateResourceRequest{Name: "Orders", Handle: "orders"})
	s.Require().NoError(err)
	_, err = createActionAtResource(serverID, resourceID, CreateActionRequest{Name: "Read", Handle: "read"})
	s.Require().NoError(err)
	status, body := s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"otherTreeRoots": map[string]interface{}{
			"mode": "selected", "units": []map[string]interface{}{{"ouId": s.otherOUID}},
		},
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)
	unsharedID := s.createServer("Not Shared With Other Tree API")

	call := s.callerWithToken(s.adminToken(s.otherOUID, s.otherOUID, "rs-sharing-other-admin"))

	status, server := call(http.MethodGet, resourceServerURL("/%s", serverID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", server)
	s.Equal(s.otherOUID, server["ouId"], "the resource server is read under the unit it is shared with")
	s.Equal(true, server["isReadOnly"])
	s.Equal(false, server["isOwnedByOU"])

	status, resources := call(http.MethodGet, resourceServerURL("/%s/resources", serverID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", resources)
	s.Equal(float64(1), resources["totalResults"])

	status, actions := call(http.MethodGet,
		resourceServerURL("/%s/resources/%s/actions", serverID, resourceID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", actions)
	s.Equal(float64(1), actions["totalResults"])

	status, body = call(http.MethodPut, resourceServerURL("/%s", serverID),
		map[string]interface{}{"name": "Renamed", "identifier": server["identifier"], "ouId": s.otherOUID})
	s.Equal(http.StatusForbidden, status, "a shared resource server cannot be changed there: %v", body)

	status, body = call(http.MethodGet, resourceServerURL("/%s", unsharedID), nil)
	s.Equal(http.StatusForbidden, status, "a resource server not shared with the unit is not read there: %v", body)
}

// callerWithToken returns a function that sends a request with the token and returns the status and the decoded body.
func (s *ResourceServerSharingTestSuite) callerWithToken(
	token string,
) func(method, url string, body interface{}) (int, map[string]interface{}) {
	client := testutils.GetHTTPClientWithToken(token)
	return func(method, url string, body interface{}) (int, map[string]interface{}) {
		s.T().Helper()
		var reader io.Reader
		if body != nil {
			raw, err := json.Marshal(body)
			s.Require().NoError(err)
			reader = bytes.NewReader(raw)
		}
		req, err := http.NewRequest(method, url, reader)
		s.Require().NoError(err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		s.Require().NoError(err)
		defer resp.Body.Close()
		decoded := map[string]interface{}{}
		_ = json.NewDecoder(resp.Body).Decode(&decoded)
		return resp.StatusCode, decoded
	}
}

// An administrator of a child unit reads and lists a resource server of the unit above it only once it is shared
// with the child, as the resource servers the child's roles can use.
func (s *ResourceServerSharingTestSuite) TestParentServerIsReadInTheChildOnlyOnceShared() {
	serverID := s.createServer("Parent Server Read In Child API")
	call := s.callerWithToken(s.adminToken(s.childOUID, s.ownerOUID, "rs-sharing-child-admin"))
	listed := func() bool {
		s.T().Helper()
		status, list := call(http.MethodGet, resourceServerURL("?limit=100"), nil)
		s.Require().Equal(http.StatusOK, status, "body: %v", list)
		items, _ := list["resourceServers"].([]interface{})
		for _, item := range items {
			if item.(map[string]interface{})["id"] == serverID {
				return true
			}
		}
		return false
	}

	status, body := call(http.MethodGet, resourceServerURL("/%s", serverID), nil)
	s.Equal(http.StatusForbidden, status, "the parent's resource server is not shared with the child: %v", body)
	s.False(listed(), "the child does not list the parent's resource server")

	status, body = s.call(http.MethodPost, policiesURL(serverID), map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]interface{}{{"ouId": s.childOUID, "reach": "unit"}},
		},
	})
	s.Require().Equal(http.StatusCreated, status, "body: %v", body)

	status, server := call(http.MethodGet, resourceServerURL("/%s", serverID), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", server)
	s.Equal(s.childOUID, server["ouId"])
	s.Equal(false, server["isOwnedByOU"])
	s.True(listed(), "the child lists the resource server shared with it")
}
