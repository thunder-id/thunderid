// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The sharing surface is mounted over the resource-server collection, so these paths are generated
// by one Mount call rather than written per resource type.
const sharingPoliciesPath = "%s/resource-servers/%s/sharing-policies"

// ---------------------------------------------------------------------------
// Request and response shapes
// ---------------------------------------------------------------------------

// OverlayRuleBody is one field's terms on a sharing policy.
type OverlayRuleBody struct {
	Editable       bool      `json:"editable"`
	Value          *[]string `json:"value,omitempty"`
	AllowedValues  *[]string `json:"allowedValues,omitempty"`
	ExcludedValues *[]string `json:"excludedValues,omitempty"`
}

// TargetEntryBody names one organization unit a policy reaches.
type TargetEntryBody struct {
	OUID        string `json:"ouId"`
	AllChildren bool   `json:"allChildren,omitempty"`
}

// TargetOUScopeBody selects which organization units a policy reaches.
type TargetOUScopeBody struct {
	AllOUs        bool              `json:"allOus,omitempty"`
	AllRoots      bool              `json:"allRoots,omitempty"`
	RootOUIDs     []string          `json:"rootOuIds,omitempty"`
	AllChildren   bool              `json:"allChildren,omitempty"`
	OUIDs         []TargetEntryBody `json:"ouIds,omitempty"`
	ExcludedOUIDs []string          `json:"excludedOuIds,omitempty"`
}

// SharingPolicyRequest is the body of a policy create or update.
type SharingPolicyRequest struct {
	InitiatingOUID string                     `json:"initiatingOuId,omitempty"`
	TargetOUScope  TargetOUScopeBody          `json:"targetOuScope"`
	OverlayRules   map[string]OverlayRuleBody `json:"overlayRules,omitempty"`
}

// SharingPolicyResponse is one policy as the API reports it.
type SharingPolicyResponse struct {
	ID             string                     `json:"id"`
	ResourceType   string                     `json:"resourceType"`
	ResourceID     string                     `json:"resourceId"`
	OwningOUID     string                     `json:"owningOuId"`
	InitiatingOUID string                     `json:"initiatingOuId"`
	Stage          string                     `json:"stage"`
	TargetOUScope  TargetOUScopeBody          `json:"targetOuScope"`
	OverlayRules   map[string]OverlayRuleBody `json:"overlayRules"`
	Declared       bool                       `json:"declared"`
	Version        int                        `json:"version"`
}

// SharingPolicyListResponse is the body of a policy listing.
type SharingPolicyListResponse struct {
	TotalResults int                     `json:"totalResults"`
	Policies     []SharingPolicyResponse `json:"policies"`
}

// OverlayRuleResolved is one resolved rule, carrying where it came from.
type OverlayRuleResolved struct {
	Editable       bool      `json:"editable"`
	Value          *[]string `json:"value,omitempty"`
	AllowedValues  *[]string `json:"allowedValues,omitempty"`
	ExcludedValues *[]string `json:"excludedValues,omitempty"`
	Source         string    `json:"source"`
}

// OverlayResolvedResponse is what one organization unit may do with a shared resource.
type OverlayResolvedResponse struct {
	OUID      string                         `json:"ouId"`
	Origin    string                         `json:"origin"`
	PolicyIDs []string                       `json:"policyIds"`
	Rules     map[string]OverlayRuleResolved `json:"rules"`
}

// ---------------------------------------------------------------------------
// Suite
// ---------------------------------------------------------------------------

// ResourceServerSharingTestSuite exercises sharing a resource server across organization units.
//
// The tree is two separate roots plus a child under the first, which is the smallest shape that
// tells apart "reached because it was named", "reached because an ancestor was" and "not reached".
type ResourceServerSharingTestSuite struct {
	suite.Suite

	ownerOUID   string // owns the resource server
	partnerOUID string // a second root, shared to
	childOUID   string // a child of the partner, reachable only by reshare
	strangerOU  string // a third root, never shared to

	serverID string
}

func TestResourceServerSharingTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceServerSharingTestSuite))
}

func (suite *ResourceServerSharingTestSuite) SetupSuite() {
	stamp := time.Now().UnixNano()

	var err error
	suite.ownerOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharing_owner_%d", stamp),
		Name:   "Sharing Owner",
	})
	suite.Require().NoError(err, "failed to create the owning organization unit")

	suite.partnerOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharing_partner_%d", stamp),
		Name:   "Sharing Partner",
	})
	suite.Require().NoError(err, "failed to create the partner organization unit")

	suite.childOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharing_partner_child_%d", stamp),
		Name:   "Sharing Partner Child",
		Parent: &suite.partnerOUID,
	})
	suite.Require().NoError(err, "failed to create the child organization unit")

	suite.strangerOU, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("sharing_stranger_%d", stamp),
		Name:   "Sharing Stranger",
	})
	suite.Require().NoError(err, "failed to create the unrelated organization unit")

	// A server with a real permission tree: bookings and billing, each with a leaf beneath it, so
	// an exclusion can withhold one branch without touching the rest.
	suite.serverID, err = createResourceServer(CreateResourceServerRequest{
		Name: fmt.Sprintf("Sharing Booking System %d", stamp),
		OUID: suite.ownerOUID,
	})
	suite.Require().NoError(err, "failed to create the resource server")

	bookings, err := createResource(suite.serverID, CreateResourceRequest{
		Name: "Bookings", Handle: "bookings",
	})
	suite.Require().NoError(err)
	_, err = createResource(suite.serverID, CreateResourceRequest{
		Name: "Refund", Handle: "refund", Parent: &bookings,
	})
	suite.Require().NoError(err)

	billing, err := createResource(suite.serverID, CreateResourceRequest{
		Name: "Billing", Handle: "billing",
	})
	suite.Require().NoError(err)
	_, err = createResource(suite.serverID, CreateResourceRequest{
		Name: "Invoice", Handle: "invoice", Parent: &billing,
	})
	suite.Require().NoError(err)
}

func (suite *ResourceServerSharingTestSuite) TearDownSuite() {
	if suite.serverID != "" {
		if err := deleteResourceServer(suite.serverID); err != nil {
			suite.T().Logf("failed to delete the resource server: %v", err)
		}
	}
	for _, ouID := range []string{suite.childOUID, suite.partnerOUID, suite.ownerOUID, suite.strangerOU} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			suite.T().Logf("failed to delete organization unit %s: %v", ouID, err)
		}
	}
}

// SetupTest clears any policy left behind, so each test starts from an unshared server.
func (suite *ResourceServerSharingTestSuite) SetupTest() {
	list, err := listSharingPolicies(suite.serverID)
	suite.Require().NoError(err)
	for _, p := range list.Policies {
		if p.Declared {
			continue
		}
		if err := deleteSharingPolicy(suite.serverID, p.ID); err != nil {
			suite.T().Logf("failed to clear policy %s: %v", p.ID, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Visibility
// ---------------------------------------------------------------------------

// Sharing a server with no overlay rule hands over the whole permission tree, which is what a
// cascade share with nothing withheld used to mean.
func (suite *ResourceServerSharingTestSuite) TestShareWithNoRuleGivesTheWholeTree() {
	policy, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)
	suite.Equal("share", policy.Stage)
	suite.Equal(suite.ownerOUID, policy.OwningOUID)

	resolved, err := getResolvedOverlay(suite.serverID, suite.partnerOUID)
	suite.Require().NoError(err)

	rule := resolved.Rules["resources"]
	suite.False(rule.Editable, "a resource server's permission tree is not editable by a sharee")
	suite.Equal("policy", rule.Source)
	suite.Nil(rule.Value, "no value means the owner's own tree")
}

// The owner holds its own server whether or not anyone has shared it, and the fields no policy
// names fall back to the type's declared defaults.
func (suite *ResourceServerSharingTestSuite) TestOwnerSeesItsOwnServerWithNoPolicy() {
	resolved, err := getResolvedOverlay(suite.serverID, suite.ownerOUID)
	suite.Require().NoError(err)
	suite.Equal("owned", resolved.Origin)
	suite.Equal("default", resolved.Rules["resources"].Source)
}

// An organization unit no policy reaches holds nothing. Answering with the declared defaults would
// read as though it held the server on default terms.
func (suite *ResourceServerSharingTestSuite) TestUnreachedOUIsGivenNoRules() {
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	resolved, err := getResolvedOverlay(suite.serverID, suite.strangerOU)
	suite.Require().NoError(err)
	suite.Equal("none", resolved.Origin)
	suite.Empty(resolved.Rules, "it cannot see the server, so there is nothing it may do with it")
	suite.Empty(resolved.PolicyIDs)
}

// ---------------------------------------------------------------------------
// Overlay rules
// ---------------------------------------------------------------------------

// "Everything except refunds" needs no wildcard: the owner's value already is everything, and one
// excluded path removes that branch and anything added beneath it later.
func (suite *ResourceServerSharingTestSuite) TestExclusionWithholdsOneBranch() {
	excluded := []string{"bookings:refund"}
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, ExcludedValues: &excluded},
		},
	})
	suite.Require().NoError(err)

	resolved, err := getResolvedOverlay(suite.serverID, suite.partnerOUID)
	suite.Require().NoError(err)
	rule := resolved.Rules["resources"]
	suite.Require().NotNil(rule.ExcludedValues)
	suite.Equal([]string{"bookings:refund"}, *rule.ExcludedValues)
}

// An explicit include list pins exactly what the sharee gets.
func (suite *ResourceServerSharingTestSuite) TestExplicitIncludeListIsPinned() {
	value := []string{"bookings", "billing:invoice"}
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, Value: &value},
		},
	})
	suite.Require().NoError(err)

	resolved, err := getResolvedOverlay(suite.serverID, suite.partnerOUID)
	suite.Require().NoError(err)
	rule := resolved.Rules["resources"]
	suite.Require().NotNil(rule.Value)
	suite.ElementsMatch(value, *rule.Value)
}

// A rule may only name permissions the server actually defines, so a typo is an authoring error
// rather than a grant that silently matches nothing.
func (suite *ResourceServerSharingTestSuite) TestRuleNamingAnUnknownPermissionIsRejected() {
	value := []string{"bookings", "nonexistent:permission"}
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, Value: &value},
		},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "400")
}

// A resource server exposes one field; naming another is refused rather than stored and ignored.
func (suite *ResourceServerSharingTestSuite) TestUnknownFieldKeyIsRejected() {
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
		OverlayRules: map[string]OverlayRuleBody{
			"assignments": {Editable: true},
		},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "400")
}

// ---------------------------------------------------------------------------
// One policy per organization unit
// ---------------------------------------------------------------------------

// A second policy for the same initiator is refused, and the refusal names the one to edit.
func (suite *ResourceServerSharingTestSuite) TestSecondPolicyForTheSameInitiatorIsRefused() {
	first, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	_, err = createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.strangerOU}},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "409")
	suite.Contains(err.Error(), first.ID, "the refusal names the policy to edit instead")
}

// ---------------------------------------------------------------------------
// Editing
// ---------------------------------------------------------------------------

// A selective policy may grow within the one-hop rule, and the response carries the next version.
func (suite *ResourceServerSharingTestSuite) TestUpdateAddsATargetRoot() {
	created, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	updated, err := updateSharingPolicy(suite.serverID, created.ID, created.Version, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID, suite.strangerOU}},
	})
	suite.Require().NoError(err)
	suite.Equal(created.Version+1, updated.Version)

	resolved, err := getResolvedOverlay(suite.serverID, suite.strangerOU)
	suite.Require().NoError(err)
	suite.Equal("policy", resolved.Rules["resources"].Source, "the added root now sees the server")
}

// An edit built on a stale version is refused, so two administrators cannot silently overwrite
// one another's carve-outs.
func (suite *ResourceServerSharingTestSuite) TestUpdateWithAStaleVersionIsRefused() {
	created, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	_, err = updateSharingPolicy(suite.serverID, created.ID, created.Version, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID, suite.strangerOU}},
	})
	suite.Require().NoError(err)

	_, err = updateSharingPolicy(suite.serverID, created.ID, created.Version, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "412")
}

// ---------------------------------------------------------------------------
// Resharing
// ---------------------------------------------------------------------------

// A sharee may pass the server on to its own children, but only within what it itself holds.
func (suite *ResourceServerSharingTestSuite) TestReshareNarrowsWithinWhatTheShareeHolds() {
	// The partner is given the bookings branch only, so billing is outside its ceiling.
	ownerValue := []string{"bookings"}
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, Value: &ownerValue},
		},
	})
	suite.Require().NoError(err)

	suite.Run("a path beneath what it holds is accepted", func() {
		narrower := []string{"bookings:refund"}
		reshare, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
			InitiatingOUID: suite.partnerOUID,
			TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: suite.childOUID}}},
			OverlayRules: map[string]OverlayRuleBody{
				"resources": {Editable: false, Value: &narrower},
			},
		})
		suite.Require().NoError(err)
		suite.Equal("reshare", reshare.Stage)
		suite.Equal(suite.partnerOUID, reshare.InitiatingOUID)

		resolved, err := getResolvedOverlay(suite.serverID, suite.childOUID)
		suite.Require().NoError(err)
		suite.Require().NotNil(resolved.Rules["resources"].Value)
		suite.Equal([]string{"bookings:refund"}, *resolved.Rules["resources"].Value)

		suite.Require().NoError(deleteSharingPolicy(suite.serverID, reshare.ID))
	})

	suite.Run("a branch it was never given is refused", func() {
		wider := []string{"billing"}
		_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
			InitiatingOUID: suite.partnerOUID,
			TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: suite.childOUID}}},
			OverlayRules: map[string]OverlayRuleBody{
				"resources": {Editable: false, Value: &wider},
			},
		})
		suite.Require().Error(err, "billing does not live beneath bookings")
		suite.Contains(err.Error(), "400")
	})
}

// An organization unit that cannot see the server cannot share it on.
func (suite *ResourceServerSharingTestSuite) TestReshareFromAnOUWithNoStandingIsRefused() {
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		InitiatingOUID: suite.strangerOU,
		TargetOUScope:  TargetOUScopeBody{AllChildren: true},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "403")
}

// A policy may only name organization units directly beneath its initiator; depth comes from the
// scope, never from naming a grandchild.
func (suite *ResourceServerSharingTestSuite) TestReshareCannotNameAUnitItDoesNotParent() {
	_, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	_, err = createSharingPolicy(suite.serverID, SharingPolicyRequest{
		InitiatingOUID: suite.partnerOUID,
		TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: suite.strangerOU}}},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "400")
}

// ---------------------------------------------------------------------------
// Deletion
// ---------------------------------------------------------------------------

// Revoking the owner's policy cuts off the sharee and everything it had passed on.
func (suite *ResourceServerSharingTestSuite) TestDeletingTheOwnerPolicyCutsOffTheChain() {
	owner, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	_, err = createSharingPolicy(suite.serverID, SharingPolicyRequest{
		InitiatingOUID: suite.partnerOUID,
		TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: suite.childOUID}}},
	})
	suite.Require().NoError(err)

	before, err := getResolvedOverlay(suite.serverID, suite.childOUID)
	suite.Require().NoError(err)
	suite.Equal("policy", before.Rules["resources"].Source)

	suite.Require().NoError(deleteSharingPolicy(suite.serverID, owner.ID))

	after, err := getResolvedOverlay(suite.serverID, suite.childOUID)
	suite.Require().NoError(err)
	suite.Equal("none", after.Origin,
		"the child loses the server when the hop above it is revoked")
	suite.Empty(after.Rules)
}

// ---------------------------------------------------------------------------
// Route scoping
// ---------------------------------------------------------------------------

// A policy id is only addressable through the resource it belongs to.
func (suite *ResourceServerSharingTestSuite) TestPolicyOfAnotherServerIsNotFound() {
	policy, err := createSharingPolicy(suite.serverID, SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().NoError(err)

	otherID, err := createResourceServer(CreateResourceServerRequest{
		Name: fmt.Sprintf("Sharing Other Server %d", time.Now().UnixNano()),
		OUID: suite.ownerOUID,
	})
	suite.Require().NoError(err)
	defer func() { _ = deleteResourceServer(otherID) }()

	_, err = getSharingPolicy(otherID, policy.ID)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "404")
}

// A sharing call against a server that does not exist answers not-found rather than recording a
// policy pointing at nothing.
func (suite *ResourceServerSharingTestSuite) TestSharingAMissingServerIsNotFound() {
	_, err := createSharingPolicy("01900000-0000-7000-8000-00000000ffff", SharingPolicyRequest{
		TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.partnerOUID}},
	})
	suite.Require().Error(err)
	suite.Contains(err.Error(), "404")
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func createSharingPolicy(serverID string, req SharingPolicyRequest) (*SharingPolicyResponse, error) {
	body, _ := json.Marshal(req)
	url := fmt.Sprintf(sharingPoliciesPath, testServerURL, serverID)

	var out SharingPolicyResponse
	if err := doSharingRequest("POST", url, bytes.NewBuffer(body), "", http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func updateSharingPolicy(
	serverID, policyID string, version int, req SharingPolicyRequest,
) (*SharingPolicyResponse, error) {
	body, _ := json.Marshal(req)
	url := fmt.Sprintf(sharingPoliciesPath, testServerURL, serverID) + "/" + policyID

	var out SharingPolicyResponse
	err := doSharingRequest("PUT", url, bytes.NewBuffer(body), strconv.Itoa(version), http.StatusOK, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func getSharingPolicy(serverID, policyID string) (*SharingPolicyResponse, error) {
	url := fmt.Sprintf(sharingPoliciesPath, testServerURL, serverID) + "/" + policyID

	var out SharingPolicyResponse
	if err := doSharingRequest("GET", url, nil, "", http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func listSharingPolicies(serverID string) (*SharingPolicyListResponse, error) {
	url := fmt.Sprintf(sharingPoliciesPath, testServerURL, serverID)

	var out SharingPolicyListResponse
	if err := doSharingRequest("GET", url, nil, "", http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func deleteSharingPolicy(serverID, policyID string) error {
	url := fmt.Sprintf(sharingPoliciesPath, testServerURL, serverID) + "/" + policyID
	return doSharingRequest("DELETE", url, nil, "", http.StatusNoContent, nil)
}

func getResolvedOverlay(serverID, ouID string) (*OverlayResolvedResponse, error) {
	url := fmt.Sprintf("%s/resource-servers/%s/overlay-rules?ouId=%s", testServerURL, serverID, ouID)

	var out OverlayResolvedResponse
	if err := doSharingRequest("GET", url, nil, "", http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// doSharingRequest performs one call and decodes it, reporting the status and body on mismatch so a
// failure names what the server actually said.
func doSharingRequest(
	method, url string, body io.Reader, ifMatch string, wantStatus int, out interface{},
) error {
	httpReq, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		httpReq.Header.Set("If-Match", `"`+ifMatch+`"`)
	}

	resp, err := testutils.GetHTTPClient().Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---------------------------------------------------------------------------
// Declared policies
// ---------------------------------------------------------------------------

// declSharedServerID is the resource server declared in the test fixtures with an allOus policy.
const declSharedServerID = "decl-rs-shared"

// DeclaredSharingPolicyTestSuite covers a policy a resource file declares: it is not stored, it is
// editable, and editing it stores the edited policy in its place.
type DeclaredSharingPolicyTestSuite struct {
	suite.Suite

	outsiderOUID string
}

func TestDeclaredSharingPolicyTestSuite(t *testing.T) {
	suite.Run(t, new(DeclaredSharingPolicyTestSuite))
}

func (suite *DeclaredSharingPolicyTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("declared_outsider_%d", time.Now().UnixNano()),
		Name:   "Declared Sharing Outsider",
	})
	suite.Require().NoError(err, "failed to create the outsider organization unit")
	suite.outsiderOUID = ouID
}

func (suite *DeclaredSharingPolicyTestSuite) TearDownSuite() {
	if suite.outsiderOUID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.outsiderOUID); err != nil {
			suite.T().Logf("failed to delete the outsider organization unit: %v", err)
		}
	}
}

// SetupTest removes any stored override, so each test starts from the file's own policy.
func (suite *DeclaredSharingPolicyTestSuite) SetupTest() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	for _, p := range list.Policies {
		if p.Declared {
			continue
		}
		if err := deleteSharingPolicy(declSharedServerID, p.ID); err != nil {
			suite.T().Logf("failed to clear stored policy %s: %v", p.ID, err)
		}
	}
}

// The file's policy is reported as declared, and reaches an organization unit that has nothing to
// do with the server's own tree.
func (suite *DeclaredSharingPolicyTestSuite) TestDeclaredPolicyIsListedAndReachesEveryOU() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	suite.Require().Len(list.Policies, 1, "the file declares exactly one policy")

	policy := list.Policies[0]
	suite.True(policy.Declared, "a policy the file declares is reported as declared")
	suite.True(policy.TargetOUScope.AllOUs)

	resolved, err := getResolvedOverlay(declSharedServerID, suite.outsiderOUID)
	suite.Require().NoError(err)
	suite.Equal("policy", resolved.Rules["resources"].Source,
		"an allOus policy reaches an organization unit in an unrelated tree")
}

// Editing a declared policy stores it, keeping its identity, and the stored row supersedes the
// file rather than appearing alongside it.
func (suite *DeclaredSharingPolicyTestSuite) TestEditingADeclaredPolicyStoresIt() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	declared := list.Policies[0]

	// Carving an organization unit out is the narrowing a blanket policy allows.
	edited, err := updateSharingPolicy(declSharedServerID, declared.ID, declared.Version,
		SharingPolicyRequest{
			TargetOUScope: TargetOUScopeBody{
				AllOUs:        true,
				ExcludedOUIDs: []string{suite.outsiderOUID},
			},
		})
	suite.Require().NoError(err)
	suite.Equal(declared.ID, edited.ID, "the policy keeps its identity across materialization")
	suite.False(edited.Declared, "an edited policy is operator-owned from here on")

	after, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	suite.Require().Len(after.Policies, 1, "the stored policy supersedes the declared one")
	suite.False(after.Policies[0].Declared)

	resolved, err := getResolvedOverlay(declSharedServerID, suite.outsiderOUID)
	suite.Require().NoError(err)
	suite.Equal("none", resolved.Origin,
		"the carved-out organization unit no longer sees the server")
	suite.Empty(resolved.Rules)
}

// Deleting the stored override reverts to what the file declares, rather than removing sharing the
// file asked for.
func (suite *DeclaredSharingPolicyTestSuite) TestDeletingTheStoredOverrideRevertsToTheFile() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	declared := list.Policies[0]

	_, err = updateSharingPolicy(declSharedServerID, declared.ID, declared.Version,
		SharingPolicyRequest{
			TargetOUScope: TargetOUScopeBody{
				AllOUs:        true,
				ExcludedOUIDs: []string{suite.outsiderOUID},
			},
		})
	suite.Require().NoError(err)
	suite.Require().NoError(deleteSharingPolicy(declSharedServerID, declared.ID))

	reverted, err := getSharingPolicy(declSharedServerID, declared.ID)
	suite.Require().NoError(err)
	suite.True(reverted.Declared, "the file's policy applies again")
	suite.Empty(reverted.TargetOUScope.ExcludedOUIDs)

	resolved, err := getResolvedOverlay(declSharedServerID, suite.outsiderOUID)
	suite.Require().NoError(err)
	suite.Equal("policy", resolved.Rules["resources"].Source)
}

// A blanket policy may only be narrowed, and only by exclusion, whether or not a file declared it.
func (suite *DeclaredSharingPolicyTestSuite) TestADeclaredBlanketPolicyCannotChangeFamily() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	declared := list.Policies[0]

	_, err = updateSharingPolicy(declSharedServerID, declared.ID, declared.Version,
		SharingPolicyRequest{
			TargetOUScope: TargetOUScopeBody{RootOUIDs: []string{suite.outsiderOUID}},
		})
	suite.Require().Error(err, "converting a blanket policy to a selective one is not an edit")
	suite.Contains(err.Error(), "400")
}

// A sharee narrows what it passes on, and cannot pass on a branch it was not given.
func (suite *DeclaredSharingPolicyTestSuite) TestShareeNarrowsTheResourcesItPassesOn() {
	child, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("declared_outsider_child_%d", time.Now().UnixNano()),
		Name:   "Declared Sharing Outsider Child",
		Parent: &suite.outsiderOUID,
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(child) }()

	value := []string{"bookings:view"}
	reshare, err := createSharingPolicy(declSharedServerID, SharingPolicyRequest{
		InitiatingOUID: suite.outsiderOUID,
		TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: child}}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, Value: &value},
		},
	})
	suite.Require().NoError(err, "a sharee may pass on part of what it holds")
	defer func() { _ = deleteSharingPolicy(declSharedServerID, reshare.ID) }()

	resolved, err := getResolvedOverlay(declSharedServerID, child)
	suite.Require().NoError(err)
	suite.Require().NotNil(resolved.Rules["resources"].Value)
	suite.Equal([]string{"bookings:view"}, *resolved.Rules["resources"].Value,
		"the child holds only the branch it was passed")
}

// A declared policy states where sharing starts; the file owns whether it exists at all, so an
// unedited one cannot be deleted outright.
func (suite *DeclaredSharingPolicyTestSuite) TestAnUneditedDeclaredPolicyCannotBeDeleted() {
	list, err := listSharingPolicies(declSharedServerID)
	suite.Require().NoError(err)
	declared := list.Policies[0]
	suite.Require().True(declared.Declared)

	err = deleteSharingPolicy(declSharedServerID, declared.ID)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "403")
}

// declWithheldServerID is the declared resource server whose policy withholds one branch.
const declWithheldServerID = "decl-rs-withheld"

// A declared policy may carry overlay rules, not just a target scope. This is the case that proves
// a withheld branch is hidden from the sharee rather than merely unusable.
type DeclaredOverlayRuleTestSuite struct {
	suite.Suite

	outsiderOUID string
}

func TestDeclaredOverlayRuleTestSuite(t *testing.T) {
	suite.Run(t, new(DeclaredOverlayRuleTestSuite))
}

func (suite *DeclaredOverlayRuleTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("declared_overlay_outsider_%d", time.Now().UnixNano()),
		Name:   "Declared Overlay Outsider",
	})
	suite.Require().NoError(err)
	suite.outsiderOUID = ouID
}

func (suite *DeclaredOverlayRuleTestSuite) TearDownSuite() {
	if suite.outsiderOUID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.outsiderOUID); err != nil {
			suite.T().Logf("failed to delete the outsider organization unit: %v", err)
		}
	}
}

// The rule the file declares is reported as the resolved rule, including its exclusions.
func (suite *DeclaredOverlayRuleTestSuite) TestDeclaredRuleReachesTheResolvedOverlay() {
	resolved, err := getResolvedOverlay(declWithheldServerID, suite.outsiderOUID)
	suite.Require().NoError(err)

	rule := resolved.Rules["resources"]
	suite.Equal("policy", rule.Source, "the declared policy decided this, not the default")
	suite.False(rule.Editable, "a resource server's permission tree is not editable by a sharee")
	suite.Require().NotNil(rule.ExcludedValues)
	suite.Contains(*rule.ExcludedValues, "orders:cancel")
}

// The policy the file declares is listed, and marked as declared until someone edits it.
func (suite *DeclaredOverlayRuleTestSuite) TestDeclaredPolicyCarriesItsRule() {
	list, err := listSharingPolicies(declWithheldServerID)
	suite.Require().NoError(err)
	suite.Require().Len(list.Policies, 1)

	policy := list.Policies[0]
	suite.True(policy.Declared)
	suite.True(policy.TargetOUScope.AllOUs)

	rule, ok := policy.OverlayRules["resources"]
	suite.Require().True(ok, "the declared rule is reported back")
	suite.Require().NotNil(rule.ExcludedValues)
	suite.Equal([]string{"orders:cancel"}, *rule.ExcludedValues)
}

// A reshare may not hand on a branch the declared policy withheld from its initiator.
func (suite *DeclaredOverlayRuleTestSuite) TestReshareCannotHandOnAWithheldBranch() {
	child, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("declared_overlay_child_%d", time.Now().UnixNano()),
		Name:   "Declared Overlay Child",
		Parent: &suite.outsiderOUID,
	})
	suite.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(child) }()

	withheld := []string{"orders:cancel"}
	_, err = createSharingPolicy(declWithheldServerID, SharingPolicyRequest{
		InitiatingOUID: suite.outsiderOUID,
		TargetOUScope:  TargetOUScopeBody{OUIDs: []TargetEntryBody{{OUID: child}}},
		OverlayRules: map[string]OverlayRuleBody{
			"resources": {Editable: false, Value: &withheld},
		},
	})
	suite.Require().Error(err, "the initiator was never given this branch")
	suite.Contains(err.Error(), "400")
}
