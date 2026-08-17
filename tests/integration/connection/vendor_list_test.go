// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
)

// connectionInstanceSummary mirrors backend/internal/connection/models.go connectionInstanceSummary,
// the shape the vendor scoped collection endpoints return.
type connectionInstanceSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// VendorListTestSuite covers the vendor scoped collection endpoints, GET /connections/{vendor},
// which list only the instances of that vendor. The unified GET /connections listing is covered by
// the connection API suite; this suite asserts the per-vendor scoping, for identity providers and
// for message providers alike.
type VendorListTestSuite struct {
	suite.Suite
	googleID     string
	oidcID       string
	twilioID     string
	smsGatewayID string
}

func TestVendorListSuite(t *testing.T) {
	suite.Run(t, new(VendorListTestSuite))
}

func (ts *VendorListTestSuite) SetupSuite() {
	ts.googleID = ts.create("google", googleConnectionRequest{
		Name:         "Vendor List Google",
		ClientID:     "vendor-list-google-client",
		ClientSecret: "vendor-list-google-secret",
		RedirectURI:  "https://localhost:8095/flow/authn",
	})
	ts.oidcID = ts.create("oidc", oidcConnectionRequest{
		Name:                  "Vendor List OIDC",
		ClientID:              "vendor-list-oidc-client",
		ClientSecret:          "vendor-list-oidc-secret",
		RedirectURI:           "https://localhost:8095/flow/authn",
		AuthorizationEndpoint: "https://vendor-list.example.com/authorize",
		TokenEndpoint:         "https://vendor-list.example.com/token",
	})
	ts.twilioID = ts.create("twilio", twilioConnectionRequest{
		Name:       "Vendor List Twilio",
		AccountSID: "AC00000000000000000000000000000001",
		AuthToken:  "vendor-list-twilio-token",
		SenderID:   "+10000000001",
	})
	ts.smsGatewayID = ts.create("sms-gateway", smsGatewayConnectionRequest{
		Name:       "Vendor List SMS Gateway",
		URL:        "https://vendor-list.example.com/sms",
		HTTPMethod: "POST",
	})
}

func (ts *VendorListTestSuite) TearDownSuite() {
	for vendor, id := range map[string]string{
		"google":      ts.googleID,
		"oidc":        ts.oidcID,
		"twilio":      ts.twilioID,
		"sms-gateway": ts.smsGatewayID,
	} {
		if id == "" {
			continue
		}
		res, err := doRequest(http.MethodDelete, "/connections/"+vendor+"/"+id, nil)
		if err != nil || res.status != http.StatusNoContent {
			ts.T().Logf("Failed to delete the %s connection during teardown: status=%d err=%v",
				vendor, res.status, err)
		}
	}
}

func (ts *VendorListTestSuite) create(vendor string, body interface{}) string {
	res, err := doRequest(http.MethodPost, "/connections/"+vendor, body)
	ts.Require().NoError(err, "Failed to create the %s connection", vendor)
	ts.Require().Equal(http.StatusCreated, res.status,
		"Creating the %s connection should return 201: %s", vendor, string(res.body))

	var created connectionResponse
	ts.Require().NoError(res.decode(&created), "Failed to decode the created %s connection", vendor)
	ts.Require().NotEmpty(created.ID, "The created %s connection must have an id", vendor)
	return created.ID
}

// listVendor returns the instances the vendor scoped collection endpoint reports.
func (ts *VendorListTestSuite) listVendor(vendor string) []connectionInstanceSummary {
	res, err := doRequest(http.MethodGet, "/connections/"+vendor, nil)
	ts.Require().NoError(err, "Failed to list the %s connections", vendor)
	ts.Require().Equal(http.StatusOK, res.status,
		"Listing the %s connections should return 200: %s", vendor, string(res.body))

	var instances []connectionInstanceSummary
	ts.Require().NoError(res.decode(&instances), "Failed to decode the %s connection list", vendor)
	return instances
}

// containsSummaryID reports whether the vendor scoped listing contains the given instance id.
func containsSummaryID(instances []connectionInstanceSummary, id string) bool {
	for _, instance := range instances {
		if instance.ID == id {
			return true
		}
	}
	return false
}

// TestIdentityProviderVendorListIsScopedToItsType asserts that an identity provider collection lists
// its own instances and none of another vendor's.
func (ts *VendorListTestSuite) TestIdentityProviderVendorListIsScopedToItsType() {
	googleInstances := ts.listVendor("google")
	ts.True(containsSummaryID(googleInstances, ts.googleID), "The google list must contain the google connection")
	ts.False(containsSummaryID(googleInstances, ts.oidcID), "The google list must not contain the OIDC connection")

	oidcInstances := ts.listVendor("oidc")
	ts.True(containsSummaryID(oidcInstances, ts.oidcID), "The OIDC list must contain the OIDC connection")
	ts.False(containsSummaryID(oidcInstances, ts.googleID), "The OIDC list must not contain the google connection")

	// A vendor with no configured instance still answers with an empty collection rather than 404.
	githubInstances := ts.listVendor("github")
	ts.False(containsSummaryID(githubInstances, ts.googleID),
		"The github list must not contain connections of other vendors")

	for _, instance := range googleInstances {
		if instance.ID == ts.googleID {
			ts.Equal("Vendor List Google", instance.Name,
				"The listed instance must carry the name it was created with")
		}
	}
}

// TestMessageProviderVendorListIsScopedToItsProvider asserts the same scoping for the message
// provider collections, which resolve through the notification sender service instead.
func (ts *VendorListTestSuite) TestMessageProviderVendorListIsScopedToItsProvider() {
	twilioInstances := ts.listVendor("twilio")
	ts.True(containsSummaryID(twilioInstances, ts.twilioID), "The twilio list must contain the twilio sender")
	ts.False(containsSummaryID(twilioInstances, ts.smsGatewayID),
		"The twilio list must not contain the custom gateway sender")

	gatewayInstances := ts.listVendor("sms-gateway")
	ts.True(containsSummaryID(gatewayInstances, ts.smsGatewayID),
		"The custom gateway list must contain the custom gateway sender")
	ts.False(containsSummaryID(gatewayInstances, ts.twilioID),
		"The custom gateway list must not contain the twilio sender")

	vonageInstances := ts.listVendor("vonage")
	ts.False(containsSummaryID(vonageInstances, ts.twilioID),
		"The vonage list must not contain senders of other providers")

	for _, instance := range twilioInstances {
		if instance.ID == ts.twilioID {
			ts.Equal("Vendor List Twilio", instance.Name,
				"The listed sender must carry the name it was created with")
		}
	}
}

// TestUnknownVendorCollectionIsNotFound asserts that only registered vendors have a collection
// endpoint, so an unknown vendor is not silently treated as an empty one.
func (ts *VendorListTestSuite) TestUnknownVendorCollectionIsNotFound() {
	res, err := doRequest(http.MethodGet, "/connections/not-a-vendor", nil)
	ts.Require().NoError(err, "Failed to send the request for an unknown vendor")
	ts.Equal(http.StatusNotFound, res.status,
		"An unknown vendor collection should return 404: %s", string(res.body))
}
