// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The connection API answers a sharing refusal in its own vocabulary: the framework's codes stay inside the
// framework, and the description carries what is wrong with the request.
const (
	errInvalidSharingPolicy        = "CON-1031"
	errSharingPolicyNotFound       = "CON-1032"
	errSharingPolicyExists         = "CON-1033"
	errSharingPolicyVersionMissing = "CON-1034"
	errConnectionNotSharedToOU     = "CON-1036"
	errMissingOUIDParameter        = "CON-1037"
	errDeploymentWideSharing       = "CON-1038"
	errIDPNotFound                 = "IDP-1001"
	errInvalidFilter               = "CON-1039"
)

// sharedVendor is a vendor whose connections can be shared, with the body that creates one.
type sharedVendor struct {
	vendor string
	body   func(name string) interface{}
}

// sharedVendors returns a vendor of every kind of connection that is shared: an identity provider, a message and an
// email provider (notification senders), the AuthZEN PDP connection and the HTTP connection.
func sharedVendors() []sharedVendor {
	var out []sharedVendor
	out = append(out, sharedVendor{
		vendor: "google",
		body:   func(name string) interface{} { return googleRequestIn(name, "") },
	})
	for _, v := range ownedNonIDPVendors() {
		out = append(out, sharedVendor{
			vendor: v.vendor,
			body:   func(name string) interface{} { return v.body(name, "") },
		})
	}
	return out
}

// sharingRequest calls a sharing endpoint and returns the status and the decoded body.
func (s *ConnectionAPITestSuite) sharingRequest(method, path string, body interface{}) (int, map[string]interface{}) {
	s.T().Helper()
	res, err := doRequest(method, path, body)
	s.Require().NoError(err)
	decoded := map[string]interface{}{}
	if len(res.body) > 0 {
		_ = res.decode(&decoded)
	}
	return res.status, decoded
}

// allChildUnitsSection is the section reaching everything beneath the issuer, less what it excludes.
func allChildUnitsSection(excluded ...string) map[string]interface{} {
	section := map[string]interface{}{"mode": "all"}
	if len(excluded) > 0 {
		section["excludedOuIds"] = excluded
	}
	return section
}

// selectedChildUnitsSection names direct children of the issuer, each with its reach.
func selectedChildUnitsSection(units ...map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"mode": "selected", "units": units}
}

func childUnitReach(ouID, reach string) map[string]interface{} {
	return map[string]interface{}{"ouId": ouID, "reach": reach}
}

// createChildOU creates an organization unit under the suite's one and removes it when the test ends.
func (s *ConnectionAPITestSuite) createChildOU(handle string) string {
	s.T().Helper()
	parent := s.ouID
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: &parent,
	})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// A sharing policy is created, read, edited and deleted through the connection that governs it, for every kind of
// connection, and the organization unit it reaches can be resolved at each step.
func (s *ConnectionAPITestSuite) TestConnectionSharingPolicyLifecycle() {
	childOUID := s.createChildOU("conn-sharing-child")

	for _, v := range sharedVendors() {
		s.Run(v.vendor, func() {
			created := s.createConnection(v.vendor, v.body("Sharing Lifecycle "+v.vendor))
			defer s.deleteConnection(v.vendor, created.ID)
			base := "/connections/" + v.vendor + "/" + created.ID + "/sharing-policies"
			overlayPath := "/connections/" + v.vendor + "/" + created.ID + "/overlay-rules?ouId=" + childOUID

			// The child cannot see the connection before a policy reaches it, so there is nothing to resolve.
			status, overlay := s.sharingRequest(http.MethodGet, overlayPath, nil)
			s.Require().Equal(http.StatusNotFound, status, "body: %v", overlay)
			s.Equal(errConnectionNotSharedToOU, overlay["code"])

			status, policy := s.sharingRequest(http.MethodPost, base,
				map[string]interface{}{"childUnits": allChildUnitsSection()})
			s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
			policyID, _ := policy["id"].(string)
			s.Require().NotEmpty(policyID)
			s.Equal("share", policy["stage"])
			s.Equal(float64(1), policy["version"])
			defer func() { _, _ = s.sharingRequest(http.MethodDelete, base+"/"+policyID, nil) }()

			status, overlay = s.sharingRequest(http.MethodGet, overlayPath, nil)
			s.Require().Equal(http.StatusOK, status, "body: %v", overlay)
			s.True(overlay["visible"].(bool), "reaching every child unit reaches the child")
			s.Equal(false, overlay["isOwnedByOU"])
			s.Empty(overlay["rules"], "a connection declares no overlay fields, so a sharee can override nothing")

			status, owner := s.sharingRequest(http.MethodGet,
				"/connections/"+v.vendor+"/"+created.ID+"/overlay-rules?ouId="+s.ouID, nil)
			s.Require().Equal(http.StatusOK, status, "body: %v", owner)
			s.Equal(true, owner["isOwnedByOU"], "the owner sees its own connection as owned")

			status, fetched := s.sharingRequest(http.MethodGet, base+"/"+policyID, nil)
			s.Require().Equal(http.StatusOK, status)
			childUnits, _ := fetched["childUnits"].(map[string]interface{})
			s.Require().NotNil(childUnits, "the policy comes back stating its sections")
			s.Equal("all", childUnits["mode"])
			s.NotContains(fetched, "allUnits", "a section the policy does not state is left out")

			status, listed := s.sharingRequest(http.MethodGet, base, nil)
			s.Require().Equal(http.StatusOK, status)
			s.GreaterOrEqual(listed["totalResults"].(float64), float64(1))

			// Excluding the child from the section withdraws it.
			status, updated := s.sharingRequest(http.MethodPut, base+"/"+policyID, map[string]interface{}{
				"version":    1,
				"childUnits": allChildUnitsSection(childOUID),
			})
			s.Require().Equal(http.StatusOK, status, "body: %v", updated)
			s.Equal(float64(2), updated["version"])

			status, overlay = s.sharingRequest(http.MethodGet, overlayPath, nil)
			s.Require().Equal(http.StatusNotFound, status, "the carve-out withdrew the child")
			s.Equal(errConnectionNotSharedToOU, overlay["code"])

			status, _ = s.sharingRequest(http.MethodDelete, base+"/"+policyID, nil)
			s.Require().Equal(http.StatusNoContent, status)
			status, _ = s.sharingRequest(http.MethodGet, base+"/"+policyID, nil)
			s.Equal(http.StatusNotFound, status, "the policy is gone")

			// A delete says what the state should be afterwards, so repeating it succeeds.
			status, _ = s.sharingRequest(http.MethodDelete, base+"/"+policyID, nil)
			s.Equal(http.StatusNoContent, status, "deleting it again is not an error")
		})
	}
}

// A policy the API refuses is answered in the connection's own codes.
func (s *ConnectionAPITestSuite) TestConnectionSharingPolicyRefusals() {
	childOUID := s.createChildOU("conn-sharing-refusals-child")
	created := s.createConnection("google", googleRequestIn("Sharing Refusals Google", ""))
	defer s.deleteConnection("google", created.ID)
	base := "/connections/google/" + created.ID + "/sharing-policies"

	for _, tc := range []struct {
		name    string
		body    map[string]interface{}
		wantErr string
	}{
		{"no section at all", map[string]interface{}{}, errInvalidSharingPolicy},
		{
			"all child units naming an organization unit",
			map[string]interface{}{"childUnits": map[string]interface{}{
				"mode": "all", "units": []map[string]interface{}{childUnitReach(childOUID, "unit")}}},
			errInvalidSharingPolicy,
		},
		{
			"a child unit with no reach",
			map[string]interface{}{"childUnits": selectedChildUnitsSection(map[string]interface{}{"ouId": childOUID})},
			errInvalidSharingPolicy,
		},
		{
			"an organization unit named twice",
			map[string]interface{}{"childUnits": selectedChildUnitsSection(
				childUnitReach(childOUID, "unit"), childUnitReach(childOUID, "subtree"))},
			errInvalidSharingPolicy,
		},
		{"all units through the API", map[string]interface{}{"allUnits": map[string]interface{}{}},
			errDeploymentWideSharing},
		{"every other tree's root through the API",
			map[string]interface{}{"otherTreeRoots": map[string]interface{}{"mode": "all"}},
			errDeploymentWideSharing},
	} {
		s.Run(tc.name, func() {
			status, body := s.sharingRequest(http.MethodPost, base, tc.body)
			s.Equal(http.StatusBadRequest, status, "body: %v", body)
			s.Equal(tc.wantErr, body["code"])
		})
	}

	// A body naming a field the policy does not declare is refused rather than read as something narrower.
	status, body := s.sharingRequest(http.MethodPost, base, map[string]interface{}{
		"childUnits": map[string]interface{}{"mode": "all", "excludedOuId": []string{childOUID}}})
	s.Equal(http.StatusBadRequest, status, "body: %v", body)
	s.Equal("CON-1007", body["code"])

	// A second policy for the same organization unit is a conflict pointing at the one to edit.
	status, policy := s.sharingRequest(http.MethodPost, base,
		map[string]interface{}{"childUnits": selectedChildUnitsSection(childUnitReach(childOUID, "unit"))})
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)
	defer func() { _, _ = s.sharingRequest(http.MethodDelete, base+"/"+policyID, nil) }()

	status, conflict := s.sharingRequest(http.MethodPost, base,
		map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Equal(http.StatusConflict, status, "body: %v", conflict)
	s.Equal(errSharingPolicyExists, conflict["code"])

	// An edit against a version that moved is refused rather than flattening the write.
	status, stale := s.sharingRequest(http.MethodPut, base+"/"+policyID, map[string]interface{}{
		"version":    99,
		"childUnits": selectedChildUnitsSection(childUnitReach(childOUID, "subtree")),
	})
	s.Equal(http.StatusConflict, status, "body: %v", stale)
	s.Equal(errSharingPolicyVersionMissing, stale["code"])

	// A policy id this connection does not own is not reachable through its path.
	status, missing := s.sharingRequest(http.MethodGet, base+"/00000000-0000-0000-0000-000000000000", nil)
	s.Equal(http.StatusNotFound, status)
	s.Equal(errSharingPolicyNotFound, missing["code"])

	// The overlay question needs an organization unit to ask about.
	status, noOU := s.sharingRequest(http.MethodGet, "/connections/google/"+created.ID+"/overlay-rules", nil)
	s.Equal(http.StatusBadRequest, status, "body: %v", noOU)
	s.Equal(errMissingOUIDParameter, noOU["code"])
}

// A policy is reachable only through the connection it governs.
func (s *ConnectionAPITestSuite) TestConnectionSharingPolicyIsScopedToItsConnection() {
	owned := s.createConnection("google", googleRequestIn("Sharing Scope Google A", ""))
	defer s.deleteConnection("google", owned.ID)
	other := s.createConnection("google", googleRequestIn("Sharing Scope Google B", ""))
	defer s.deleteConnection("google", other.ID)

	status, policy := s.sharingRequest(http.MethodPost, "/connections/google/"+owned.ID+"/sharing-policies",
		map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)

	status, body := s.sharingRequest(http.MethodGet,
		"/connections/google/"+other.ID+"/sharing-policies/"+policyID, nil)
	s.Equal(http.StatusNotFound, status, "body: %v", body)
	s.Equal(errSharingPolicyNotFound, body["code"])

	status, body = s.sharingRequest(http.MethodPut,
		"/connections/google/"+other.ID+"/sharing-policies/"+policyID,
		map[string]interface{}{"version": 1, "childUnits": allChildUnitsSection()})
	s.Equal(http.StatusNotFound, status, "body: %v", body)

	status, _ = s.sharingRequest(http.MethodDelete, "/connections/google/"+owned.ID+"/sharing-policies/"+policyID, nil)
	s.Equal(http.StatusNoContent, status)
}

// A connection that does not exist has no sharing to read or write.
func (s *ConnectionAPITestSuite) TestConnectionSharingOnUnknownConnection() {
	base := "/connections/google/" + unknownOUID + "/sharing-policies"

	status, body := s.sharingRequest(http.MethodPost, base, map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Equal(http.StatusNotFound, status, "body: %v", body)
	s.Equal(errIDPNotFound, body["code"])

	status, body = s.sharingRequest(http.MethodGet, base, nil)
	s.Equal(http.StatusNotFound, status, "body: %v", body)
}

// A delete the connection's usages refuse keeps its sharing policies: the connection is still there.
func (s *ConnectionAPITestSuite) TestBlockedConnectionDeleteKeepsItsSharingPolicies() {
	created := s.createConnection("email-smtp", smtpConnectionRequest{
		Name: "Sharing Blocked SMTP", Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
	})
	defer s.deleteConnection("email-smtp", created.ID)
	path := "/connections/email-smtp/" + created.ID

	flowID, err := testutils.CreateFlow(emailOTPFlowUsingSender("connection-sharing-blocked-flow", created.ID))
	s.Require().NoError(err)
	flowDeleted := false
	defer func() {
		if !flowDeleted {
			_ = testutils.DeleteFlow(flowID)
		}
	}()

	status, policy := s.sharingRequest(http.MethodPost, path+"/sharing-policies",
		map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)

	res, err := doRequest(http.MethodDelete, path, nil)
	s.Require().NoError(err)
	s.Equal(http.StatusConflict, res.status, string(res.body))
	s.Contains(string(res.body), "MNS-1016")

	status, _ = s.sharingRequest(http.MethodGet, path+"/sharing-policies/"+policyID, nil)
	s.Equal(http.StatusOK, status, "the refused delete left the connection's sharing in place")

	s.Require().NoError(testutils.DeleteFlow(flowID))
	flowDeleted = true
}

// Deleting a connection takes its sharing policies with it. A policy left behind would still count as issued by
// the owning organization unit, which would then refuse to be deleted.
func (s *ConnectionAPITestSuite) TestDeletingAConnectionRemovesItsSharingPolicies() {
	for _, v := range sharedVendors() {
		s.Run(v.vendor, func() {
			parentID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
				Handle: "conn-sharing-cascade-" + v.vendor, Name: "Sharing Cascade " + v.vendor,
			})
			s.Require().NoError(err)
			s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(parentID) })
			childID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
				Handle: "conn-sharing-cascade-child-" + v.vendor, Name: "Sharing Cascade Child " + v.vendor,
				Parent: &parentID,
			})
			s.Require().NoError(err)
			s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(childID) })

			res, err := doRequest(http.MethodPost, "/connections/"+v.vendor, withOU(s, v.body("Sharing Cascade "+v.vendor), parentID))
			s.Require().NoError(err)
			s.Require().Equal(http.StatusCreated, res.status, string(res.body))
			var created connectionResponse
			s.Require().NoError(res.decode(&created))
			s.T().Cleanup(func() { _, _ = doRequest(http.MethodDelete, "/connections/"+v.vendor+"/"+created.ID, nil) })

			status, policy := s.sharingRequest(http.MethodPost,
				"/connections/"+v.vendor+"/"+created.ID+"/sharing-policies",
				map[string]interface{}{"childUnits": allChildUnitsSection()})
			s.Require().Equal(http.StatusCreated, status, "body: %v", policy)

			s.deleteConnection(v.vendor, created.ID)

			s.Require().NoError(testutils.DeleteOrganizationUnit(childID))
			s.Require().NoError(testutils.DeleteOrganizationUnit(parentID),
				"the owning organization unit still holds a policy the deleted connection issued")
		})
	}
}

// withOU returns the request body with the owning organization unit set.
func withOU(s *ConnectionAPITestSuite, body interface{}, ouID string) interface{} {
	s.T().Helper()
	object, ok := ownedBy(s.T(), ouID, body).(map[string]interface{})
	s.Require().True(ok)
	object["ouId"] = ouID
	return object
}

// A delete on the path of a vendor that does not hold the connection must leave its sharing alone. Some services
// answer such a delete with success, which makes it easy to mistake for a connection that is gone.
func (s *ConnectionAPITestSuite) TestDeleteOnAnotherVendorKeepsSharingPolicies() {
	created := s.createConnection("google", googleRequestIn("Sharing Wrong Vendor Google", ""))
	defer s.deleteConnection("google", created.ID)
	policies := "/connections/google/" + created.ID + "/sharing-policies"

	status, policy := s.sharingRequest(http.MethodPost, policies,
		map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	policyID, _ := policy["id"].(string)

	for _, vendor := range []string{"http", "twilio", "email-smtp", "authzen-pdp", "github"} {
		s.Run(vendor, func() {
			res, err := doRequest(http.MethodDelete, "/connections/"+vendor+"/"+created.ID, nil)
			s.Require().NoError(err)
			s.Require().Contains([]int{http.StatusNoContent, http.StatusNotFound}, res.status, string(res.body))

			status, body := s.sharingRequest(http.MethodGet, policies+"/"+policyID, nil)
			s.Equal(http.StatusOK, status, "the connection's sharing survived a delete on /connections/%s: %v",
				vendor, body)
		})
	}
}

// listedIn lists the connections of an organization unit through the ouId filter and returns each entry by id.
func (s *ConnectionAPITestSuite) listedIn(ouID string) map[string]map[string]interface{} {
	s.T().Helper()
	status, body := s.sharingRequest(http.MethodGet,
		"/connections?limit=100&filter="+url.QueryEscape(`ouId eq "`+ouID+`"`), nil)
	s.Require().Equal(http.StatusOK, status, "body: %v", body)
	byID := map[string]map[string]interface{}{}
	for _, item := range body["connections"].([]interface{}) {
		entry := item.(map[string]interface{})
		byID[entry["id"].(string)] = entry
	}
	return byID
}

func (s *ConnectionAPITestSuite) TestTheOUFilterListsOwnedConnectionsThenSharedOnes() {
	childOUID := s.createChildOU("conn-filter-child")
	created := s.createConnection("google", googleRequestIn("Filter Owner", ""))
	defer s.deleteConnection("google", created.ID)

	owned := s.listedIn(s.ouID)
	s.Require().Contains(owned, created.ID, "the owner lists the connection it owns")
	s.Equal(true, owned[created.ID]["isOwnedByOU"])
	s.NotContains(s.listedIn(childOUID), created.ID, "nothing was shared with the child yet")

	base := "/connections/google/" + created.ID + "/sharing-policies"
	status, policy := s.sharingRequest(http.MethodPost, base,
		map[string]interface{}{"childUnits": allChildUnitsSection()})
	s.Require().Equal(http.StatusCreated, status, "body: %v", policy)
	defer func() { _, _ = s.sharingRequest(http.MethodDelete, base+"/"+policy["id"].(string), nil) }()

	shared := s.listedIn(childOUID)
	s.Require().Contains(shared, created.ID, "the connection shared with the child is listed for it")
	s.Equal(false, shared[created.ID]["isOwnedByOU"])
	s.Equal(s.ouID, shared[created.ID]["ouId"], "the owner's, not the one listed")

	status, body := s.sharingRequest(http.MethodGet, "/connections?limit=100", nil)
	s.Require().Equal(http.StatusOK, status)
	for _, item := range body["connections"].([]interface{}) {
		s.Equal(true, item.(map[string]interface{})["isOwnedByOU"], "an unfiltered list is of owned connections")
	}
}

func (s *ConnectionAPITestSuite) TestAConnectionFilterItCannotApplyIsRefused() {
	for _, expr := range []string{
		"ouId eq " + s.ouID,
		`ouId eq ""`,
		`ouId sw "` + s.ouID + `"`,
		`name eq "x"`,
		`ouId eq "` + s.ouID + `" and name eq "x"`,
	} {
		status, body := s.sharingRequest(http.MethodGet, "/connections?filter="+url.QueryEscape(expr), nil)
		s.Equal(http.StatusBadRequest, status, expr)
		s.Equal(errInvalidFilter, body["code"], expr)
	}
}
