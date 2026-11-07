// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// Declarative fixture loaded from tests/integration/resources/declarative_resources/authorization_detail_types.
	declarativeTypeID    = "decl-adt-1"
	declarativeTypeValue = "https://declarative.example.com/flight"

	// codeImmutableType is returned for a write to a type declared in a file.
	codeImmutableType = "RAR-1013"
)

// DeclarativeTypeTestSuite covers a type declared in a file, served next to registered types by the
// composite store.
type DeclarativeTypeTestSuite struct {
	suite.Suite
}

func TestDeclarativeTypeTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeTypeTestSuite))
}

func (s *DeclarativeTypeTestSuite) TestGetShowsTheDeclaredTypeAsReadOnly() {
	status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+declarativeTypeID, nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var declared typeResponse
	s.Require().NoError(json.Unmarshal(body, &declared))
	s.Equal(declarativeTypeValue, declared.Type)
	s.Equal("Declarative Flight", declared.Name)
	s.Equal("Book a {{route}} flight", declared.ConsentTemplate)
	s.True(declared.Reuse)
	s.True(declared.IsReadOnly, "a declared type is read-only")
	s.JSONEq(`{"type":"object","required":["route"],"properties":{"route":{"type":"string","title":"Route"}}}`,
		string(declared.Schema))
}

func (s *DeclarativeTypeTestSuite) TestListMergesDeclaredAndRegisteredTypes() {
	registered := createType(s.T(), typeRequest{Type: uniqueTypeValue("merged"), Name: "Registered",
		Schema: flightSchema})
	defer deleteType(s.T(), registered.ID)

	all := listAllTypes(s.T())
	s.Contains(typeIDs(all), declarativeTypeID)
	s.Contains(typeIDs(all), registered.ID)
}

func (s *DeclarativeTypeTestSuite) TestWritesToTheDeclaredTypeAreRefused() {
	item := importType(s.T(), declarativeTypeID, typeRequest{Type: declarativeTypeValue, Name: "Renamed",
		Schema: flightSchema})
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeImmutableType, item.Code)

	item, err := testutils.DeleteImportedResourceOutcome(typeResourceType, declarativeTypeID)
	s.Require().NoError(err)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeImmutableType, item.Code)

	status, _ := doJSON(s.T(), http.MethodGet, typesPath+"/"+declarativeTypeID, nil)
	s.Equal(http.StatusOK, status, "the declared type is still there")
}

func (s *DeclarativeTypeTestSuite) TestRegisteringTheDeclaredValueIsAConflict() {
	item := importType(s.T(), "", typeRequest{Type: declarativeTypeValue, Name: "Duplicate", Schema: flightSchema})
	deleteIfImported(s.T(), item)
	s.Equal("failed", item.Status, item.Message)
	s.Equal(codeTypeConflict, item.Code)
}

func (s *DeclarativeTypeTestSuite) TestMetadataServesTheDeclaredType() {
	status, _, body := getPublic(s.T(), metadataPath+"/"+url.PathEscape(declarativeTypeValue), nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var metadata typeMetadata
	s.Require().NoError(json.Unmarshal(body, &metadata))
	s.Equal(declarativeTypeValue, metadata.Type)
	s.NotContains(string(body), "Book a {{route}} flight", "the consent template is not published")
}
