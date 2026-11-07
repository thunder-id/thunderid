// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// codeInvalidTypeOU is returned for an organization unit that does not exist.
	codeInvalidTypeOU = "RAR-1030"
	// codeOUUpdateNotAllowed is returned when an update names another organization unit for a type.
	codeOUUpdateNotAllowed = "RAR-1031"
	// codeTypeOURequired is returned when a type is created without an organization unit.
	codeTypeOURequired = "RAR-1032"
	// codeOUHasDependents is returned when an organization unit that owns resources is deleted.
	codeOUHasDependents = "OU-1006"
	// unknownOUID names no organization unit.
	unknownOUID = "01900000-0000-7000-8000-0000000000ff"
)

// TypeOwnerOUTestSuite covers the organization unit that owns a type: it is required on create, it has to exist,
// and it cannot change afterwards. The owner is only recorded and does not restrict which clients request the type.
type TypeOwnerOUTestSuite struct {
	suite.Suite
	firstOUID  string
	secondOUID string
}

func TestTypeOwnerOUTestSuite(t *testing.T) {
	suite.Run(t, new(TypeOwnerOUTestSuite))
}

func (s *TypeOwnerOUTestSuite) SetupSuite() {
	s.firstOUID = s.createOU("adt-owner-first")
	s.secondOUID = s.createOU("adt-owner-second")
}

func (s *TypeOwnerOUTestSuite) TearDownSuite() {
	for _, ouID := range []string{s.firstOUID, s.secondOUID} {
		if ouID == "" {
			continue
		}
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			s.T().Logf("teardown: failed to delete organization unit %s: %v", ouID, err)
		}
	}
}

// createOU creates an organization unit with a handle of its own and returns its id.
func (s *TypeOwnerOUTestSuite) createOU(name string) string {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("%s-%d", name, time.Now().UnixNano()), Name: name,
	})
	s.Require().NoError(err)
	return ouID
}

// expectError sends a request and checks the status and error code it is refused with.
func (s *TypeOwnerOUTestSuite) expectError(method, path string, payload interface{}, status int, code string) {
	got, body := doJSON(s.T(), method, path, payload)
	if method == http.MethodPost {
		deleteIfCreated(s.T(), got, body)
	}
	s.Equal(status, got, string(body))
	s.Equal(code, errorCode(s.T(), body), string(body))
}

func (s *TypeOwnerOUTestSuite) TestCreate_RequiresAnExistingOU() {
	s.expectError(http.MethodPost, typesPath,
		typeRequest{Type: uniqueTypeValue("owner-missing"), Name: "No owner", Schema: flightSchema},
		http.StatusBadRequest, codeTypeOURequired)
	s.expectError(http.MethodPost, typesPath,
		typeRequest{Type: uniqueTypeValue("owner-unknown"), Name: "Unknown owner", Schema: flightSchema,
			OUID: unknownOUID},
		http.StatusBadRequest, codeInvalidTypeOU)
}

func (s *TypeOwnerOUTestSuite) TestCreate_OwnedByTheOUItNames() {
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("owner"), Name: "Owned",
		Schema: flightSchema, OUID: s.firstOUID})
	defer deleteType(s.T(), created.ID)
	s.Equal(s.firstOUID, created.OUID)

	status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+created.ID, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var fetched typeResponse
	s.Require().NoError(json.Unmarshal(body, &fetched))
	s.Equal(s.firstOUID, fetched.OUID)

	listed := false
	for _, t := range listAllTypes(s.T()) {
		if t.ID == created.ID {
			listed = true
			s.Equal(s.firstOUID, t.OUID, "the list shows the owner")
		}
	}
	s.True(listed)
}

func (s *TypeOwnerOUTestSuite) TestUpdate_KeepsTheOU() {
	value := uniqueTypeValue("owner-update")
	created := createType(s.T(), typeRequest{Type: value, Name: "Owned", Schema: flightSchema,
		OUID: s.firstOUID})
	defer deleteType(s.T(), created.ID)

	updated := updateType(s.T(), created.ID, typeRequest{Type: value, Name: "Renamed", Schema: flightSchema})
	s.Equal(s.firstOUID, updated.OUID, "an update that leaves the organization unit out keeps it")

	updated = updateType(s.T(), created.ID, typeRequest{Type: value, Name: "Renamed again", Schema: flightSchema,
		OUID: s.firstOUID})
	s.Equal(s.firstOUID, updated.OUID, "repeating the organization unit the type has keeps it")

	s.expectError(http.MethodPut, typesPath+"/"+created.ID,
		typeRequest{Type: value, Name: "Moved", Schema: flightSchema, OUID: s.secondOUID},
		http.StatusBadRequest, codeOUUpdateNotAllowed)
	s.expectError(http.MethodPut, typesPath+"/"+created.ID,
		typeRequest{Type: value, Name: "Moved", Schema: flightSchema, OUID: unknownOUID},
		http.StatusBadRequest, codeOUUpdateNotAllowed)
}

func (s *TypeOwnerOUTestSuite) TestDeleteOU_RefusedWhileItOwnsAType() {
	ouID := s.createOU("adt-owner-delete")
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("owner-delete"), Name: "Owned",
		Schema: flightSchema, OUID: ouID})

	status, body := doJSON(s.T(), http.MethodDelete, "/organization-units/"+ouID, nil)
	s.Equal(http.StatusBadRequest, status, string(body))
	s.Equal(codeOUHasDependents, errorCode(s.T(), body))

	deleteType(s.T(), created.ID)
	status, body = doJSON(s.T(), http.MethodDelete, "/organization-units/"+ouID, nil)
	s.Equal(http.StatusNoContent, status, string(body))
}
