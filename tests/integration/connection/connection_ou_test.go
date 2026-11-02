// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// unknownOUID is the id of an organization unit that does not exist.
const unknownOUID = "00000000-0000-7000-8000-00000000dead"

// googleRequestIn is a request to create a Google connection owned by ouID.
func googleRequestIn(name, ouID string) googleConnectionRequest {
	return googleConnectionRequest{
		Name: name, OUID: ouID, ClientID: "g-client", ClientSecret: "g-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	}
}

func (s *ConnectionAPITestSuite) TestConnectionRequiresOU() {
	res, err := doRequest(http.MethodPost, "/connections/google", googleRequestIn("OU Missing Google", ""))
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status, string(res.body))
	s.Equal("IDP-1016", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestConnectionNamesOU() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "conn-ou-tenant", Name: "Connection OU Tenant",
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	created := s.createConnection("google", googleRequestIn("OU Tenant Google", ouID))
	defer s.deleteConnection("google", created.ID)
	s.Equal(ouID, created.OUID)

	res, err := doRequest(http.MethodGet, "/connections/google/"+created.ID, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, res.status, string(res.body))
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Equal(ouID, fetched.OUID)

	listRes, err := doRequest(http.MethodGet, "/connections?category=identity-provider", nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, listRes.status, string(listRes.body))
	var list connectionListResponse
	s.Require().NoError(listRes.decode(&list))
	var listed *connectionInstance
	for i := range list.Connections {
		if list.Connections[i].ID == created.ID {
			listed = &list.Connections[i]
		}
	}
	s.Require().NotNil(listed, "the new connection must be listed")
	s.Equal(ouID, listed.OUID)
}

func (s *ConnectionAPITestSuite) TestConnectionWithUnknownOURefused() {
	res, err := doRequest(http.MethodPost, "/connections/google", googleRequestIn("OU Unknown Google", unknownOUID))
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status, string(res.body))
	s.Equal("IDP-1014", res.errorCode())
}

func (s *ConnectionAPITestSuite) TestConnectionOUCannotChange() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "conn-ou-immutable", Name: "Connection OU Immutable",
	})
	s.Require().NoError(err)
	defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

	created := s.createConnection("google", googleRequestIn("OU Immutable Google", ouID))
	defer s.deleteConnection("google", created.ID)

	keep, err := doRequest(http.MethodPut, "/connections/google/"+created.ID,
		googleRequestIn("OU Immutable Google 2", ""))
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, keep.status, string(keep.body))
	var kept connectionResponse
	s.Require().NoError(keep.decode(&kept))
	s.Equal(ouID, kept.OUID, "an update that leaves the ouId out keeps the one the connection has")

	same, err := doRequest(http.MethodPut, "/connections/google/"+created.ID,
		googleRequestIn("OU Immutable Google 3", ouID))
	s.Require().NoError(err)
	s.Equal(http.StatusOK, same.status, string(same.body))

	other, err := doRequest(http.MethodPut, "/connections/google/"+created.ID,
		googleRequestIn("OU Immutable Google 4", unknownOUID))
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, other.status, string(other.body))
	s.Equal("IDP-1015", other.errorCode())
}

// twilioRequestIn is a request to create a Twilio connection owned by ouID.
func twilioRequestIn(name, ouID string) twilioConnectionRequest {
	return twilioConnectionRequest{
		Name: name, OUID: ouID, AccountSID: "AC00000000000000000000000000000000",
		AuthToken: "tw-token", SenderID: "+15005550006",
	}
}

// ownedVendor describes one vendor's create request and the error codes its family answers an owner problem with.
type ownedVendor struct {
	vendor       string
	category     string
	body         func(name, ouID string) interface{}
	requiredCode string
	unknownCode  string
	changedCode  string
}

// ownedNonIDPVendors returns a vendor of every kind of connection that is not an identity provider: the message and
// email providers (notification senders), the HTTP connection and the AuthZEN PDP connection.
func ownedNonIDPVendors() []ownedVendor {
	return []ownedVendor{
		{
			vendor: "twilio", category: "sms-provider",
			body:         func(name, ouID string) interface{} { return twilioRequestIn(name, ouID) },
			requiredCode: "MNS-1020", unknownCode: "MNS-1018", changedCode: "MNS-1019",
		},
		{
			vendor: "sms-gateway", category: "sms-provider",
			body: func(name, ouID string) interface{} {
				return smsGatewayConnectionRequest{
					Name: name, OUID: ouID, URL: "https://sms.example.com/send", HTTPMethod: "POST",
				}
			},
			requiredCode: "MNS-1020", unknownCode: "MNS-1018", changedCode: "MNS-1019",
		},
		{
			vendor: "email-smtp", category: "email-provider",
			body: func(name, ouID string) interface{} {
				return smtpConnectionRequest{
					Name: name, OUID: ouID, Host: "smtp.example.com", Port: 587, FromAddress: "noreply@example.com",
				}
			},
			requiredCode: "MNS-1020", unknownCode: "MNS-1018", changedCode: "MNS-1019",
		},
		{
			vendor: "http", category: "http",
			body: func(name, ouID string) interface{} {
				return httpConnRequest{Name: name, OUID: ouID, URL: "https://ext.example.com/evaluate"}
			},
			requiredCode: "CON-1030", unknownCode: "CON-1028", changedCode: "CON-1029",
		},
		{
			vendor: "authzen-pdp", category: "authorization-pdp",
			body: func(name, ouID string) interface{} {
				body := map[string]interface{}{
					"name": name, "endpoint": "https://pdp.example.com/access/v1/evaluation",
				}
				if ouID != "" {
					body["ouId"] = ouID
				}
				return body
			},
			requiredCode: "CON-1025", unknownCode: "CON-1023", changedCode: "CON-1024",
		},
	}
}

// createOU creates an organization unit for a test and removes it when the test ends.
func (s *ConnectionAPITestSuite) createOU(handle string) string {
	s.T().Helper()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{Handle: handle, Name: handle})
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// deleteIfCreated removes the connection a request created when it succeeded although the test expected it not to.
func (s *ConnectionAPITestSuite) deleteIfCreated(vendor string, res httpResult) {
	if res.status != http.StatusCreated {
		return
	}
	var created connectionResponse
	if res.decode(&created) == nil && created.ID != "" {
		s.deleteConnection(vendor, created.ID)
	}
}

func (s *ConnectionAPITestSuite) TestNonIDPConnectionOUIDRoundTrips() {
	ouID := s.createOU("conn-ou-roundtrip")

	for _, vendor := range ownedNonIDPVendors() {
		s.Run(vendor.vendor, func() {
			created := s.createConnection(vendor.vendor, vendor.body("OU Roundtrip "+vendor.vendor, ouID))
			defer s.deleteConnection(vendor.vendor, created.ID)
			s.Equal(ouID, created.OUID, "the create response names the owning organization unit")

			res, err := doRequest(http.MethodGet, "/connections/"+vendor.vendor+"/"+created.ID, nil)
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, res.status, string(res.body))
			var fetched connectionResponse
			s.Require().NoError(res.decode(&fetched))
			s.Equal(ouID, fetched.OUID, "the vendor read names the owning organization unit")

			listRes, err := doRequest(http.MethodGet, "/connections?category="+vendor.category+"&limit=100", nil)
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, listRes.status, string(listRes.body))
			var list connectionListResponse
			s.Require().NoError(listRes.decode(&list))
			var listed *connectionInstance
			for i := range list.Connections {
				if list.Connections[i].ID == created.ID {
					listed = &list.Connections[i]
				}
			}
			s.Require().NotNil(listed, "the new connection must be listed")
			s.Equal(ouID, listed.OUID, "the listing names the owning organization unit")
		})
	}
}

func (s *ConnectionAPITestSuite) TestNonIDPConnectionRequiresOU() {
	for _, vendor := range ownedNonIDPVendors() {
		s.Run(vendor.vendor, func() {
			res, err := doRequest(http.MethodPost, "/connections/"+vendor.vendor,
				vendor.body("OU Missing "+vendor.vendor, ""))
			s.Require().NoError(err)
			s.deleteIfCreated(vendor.vendor, res)
			s.Equal(http.StatusBadRequest, res.status, string(res.body))
			s.Equal(vendor.requiredCode, res.errorCode())
		})
	}
}

func (s *ConnectionAPITestSuite) TestNonIDPConnectionWithUnknownOURefused() {
	for _, vendor := range ownedNonIDPVendors() {
		s.Run(vendor.vendor, func() {
			res, err := doRequest(http.MethodPost, "/connections/"+vendor.vendor,
				vendor.body("OU Unknown "+vendor.vendor, unknownOUID))
			s.Require().NoError(err)
			s.deleteIfCreated(vendor.vendor, res)
			s.Equal(http.StatusBadRequest, res.status, string(res.body))
			s.Equal(vendor.unknownCode, res.errorCode())
		})
	}
}

func (s *ConnectionAPITestSuite) TestNonIDPConnectionOUCannotChange() {
	ouID := s.createOU("conn-ou-immutable")
	otherOUID := s.createOU("conn-ou-immutable-other")

	for _, vendor := range ownedNonIDPVendors() {
		s.Run(vendor.vendor, func() {
			name := "OU Immutable " + vendor.vendor
			created := s.createConnection(vendor.vendor, vendor.body(name, ouID))
			defer s.deleteConnection(vendor.vendor, created.ID)
			path := "/connections/" + vendor.vendor + "/" + created.ID

			keep, err := doRequest(http.MethodPut, path, vendor.body(name+" 2", ""))
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, keep.status, string(keep.body))
			var kept connectionResponse
			s.Require().NoError(keep.decode(&kept))
			s.Equal(ouID, kept.OUID, "an update that leaves the ouId out keeps the one the connection has")

			same, err := doRequest(http.MethodPut, path, vendor.body(name+" 3", ouID))
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, same.status, string(same.body))

			other, err := doRequest(http.MethodPut, path, vendor.body(name+" 4", otherOUID))
			s.Require().NoError(err)
			s.Equal(http.StatusBadRequest, other.status, string(other.body))
			s.Equal(vendor.changedCode, other.errorCode())

			after, err := doRequest(http.MethodGet, path, nil)
			s.Require().NoError(err)
			s.Require().Equal(http.StatusOK, after.status, string(after.body))
			var current connectionResponse
			s.Require().NoError(after.decode(&current))
			s.Equal(ouID, current.OUID, "a refused update must leave the owner alone")
		})
	}
}

func (s *ConnectionAPITestSuite) TestOUWithConnectionsCannotBeDeleted() {
	owned := append([]ownedVendor{{
		vendor: "google",
		body:   func(name, ouID string) interface{} { return googleRequestIn(name, ouID) },
	}}, ownedNonIDPVendors()...)

	for _, vendor := range owned {
		s.Run(vendor.vendor, func() {
			ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
				Handle: "conn-ou-delete-" + vendor.vendor, Name: "Connection OU Delete " + vendor.vendor,
			})
			s.Require().NoError(err)
			defer func() { _ = testutils.DeleteOrganizationUnit(ouID) }()

			created := s.createConnection(vendor.vendor, vendor.body("OU Delete "+vendor.vendor, ouID))
			deleted := false
			defer func() {
				if !deleted {
					s.deleteConnection(vendor.vendor, created.ID)
				}
			}()

			refused, err := doRequest(http.MethodDelete, "/organization-units/"+ouID, nil)
			s.Require().NoError(err)
			s.Equal(http.StatusBadRequest, refused.status, string(refused.body))
			s.Equal("OU-1006", refused.errorCode())

			s.deleteConnection(vendor.vendor, created.ID)
			deleted = true

			allowed, err := doRequest(http.MethodDelete, "/organization-units/"+ouID, nil)
			s.Require().NoError(err)
			s.Equal(http.StatusNoContent, allowed.status, string(allowed.body))
		})
	}
}
