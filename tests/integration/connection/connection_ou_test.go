// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const unknownOUID = "00000000-0000-7000-8000-00000000dead"

func googleRequestIn(name, ouID string) googleConnectionRequest {
	return googleConnectionRequest{
		Name: name, OUID: ouID, ClientID: "g-client", ClientSecret: "g-secret",
		RedirectURI: "https://localhost:3000/google/callback",
	}
}

// TestConnectionDefaultsToCallerOU gives a connection created without an organization unit the caller's, and lists
// it with it.
func (s *ConnectionAPITestSuite) TestConnectionDefaultsToCallerOU() {
	created := s.createConnection("google", googleRequestIn("OU Default Google", ""))
	defer s.deleteConnection("google", created.ID)
	s.NotEmpty(created.OUID, "a connection created without an ouId is owned by the caller's organization unit")

	res, err := doRequest(http.MethodGet, "/connections/google/"+created.ID, nil)
	s.Require().NoError(err)
	s.Require().Equal(http.StatusOK, res.status, string(res.body))
	var fetched connectionResponse
	s.Require().NoError(res.decode(&fetched))
	s.Equal(created.OUID, fetched.OUID)

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
	s.Equal(created.OUID, listed.OUID)
}

// TestConnectionNamesOU records the organization unit the request names, whatever the caller's is. It is only a value:
// the connection is read back the same way for any caller.
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
}

// TestConnectionWithUnknownOURefused refuses an organization unit that does not exist (IDP-1014).
func (s *ConnectionAPITestSuite) TestConnectionWithUnknownOURefused() {
	res, err := doRequest(http.MethodPost, "/connections/google", googleRequestIn("OU Unknown Google", unknownOUID))
	s.Require().NoError(err)
	s.Equal(http.StatusBadRequest, res.status, string(res.body))
	s.Equal("IDP-1014", res.errorCode())
}

// TestConnectionOUCannotChange keeps the owner on an update that leaves it out, accepts the one it has, and refuses
// another (IDP-1015).
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
