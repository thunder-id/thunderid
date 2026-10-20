// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	metadataPath  = "/oauth2/authorization-details-types"
	discoveryPath = "/.well-known/oauth-authorization-server"

	// codeMetadataTypeNotFound is returned for a type value the metadata endpoint does not know.
	codeMetadataTypeNotFound = "RAR-1029"
)

type TypeMetadataTestSuite struct {
	suite.Suite
	typeID    string
	typeValue string
}

func TestTypeMetadataTestSuite(t *testing.T) {
	suite.Run(t, new(TypeMetadataTestSuite))
}

func (s *TypeMetadataTestSuite) SetupSuite() {
	s.typeValue = uniqueTypeValue("metadata")
	created := createType(s.T(), typeRequest{Type: s.typeValue, Name: "Metadata", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}", Processor: &processorConfig{Endpoint: "https://processor.example.com/rar"}})
	s.typeID = created.ID
}

func (s *TypeMetadataTestSuite) TearDownSuite() {
	if s.typeID != "" {
		deleteType(s.T(), s.typeID)
	}
}

func (s *TypeMetadataTestSuite) TestDiscoveryAdvertisesTheTypes() {
	status, _, body := getPublic(s.T(), discoveryPath, nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var metadata struct {
		TypesSupported []string `json:"authorization_details_types_supported"`
		TypesEndpoint  string   `json:"authorization_details_types_endpoint"`
	}
	s.Require().NoError(json.Unmarshal(body, &metadata))
	s.Contains(metadata.TypesSupported, s.typeValue)
	s.True(strings.HasSuffix(metadata.TypesEndpoint, metadataPath), metadata.TypesEndpoint)
}

func (s *TypeMetadataTestSuite) TestListShowsTheRequestViewOnly() {
	status, _, body := getPublic(s.T(), metadataPath, nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var list typeMetadataList
	s.Require().NoError(json.Unmarshal(body, &list))
	var raw struct {
		Types []json.RawMessage `json:"types"`
	}
	s.Require().NoError(json.Unmarshal(body, &raw))
	s.Require().Len(raw.Types, len(list.Types))
	var found *typeMetadata
	var foundJSON string
	for i := range list.Types {
		if list.Types[i].Type == s.typeValue {
			found = &list.Types[i]
			foundJSON = string(raw.Types[i])
		}
	}
	s.Require().NotNil(found, "the registered type is listed")
	s.Equal("Metadata", found.Name)
	s.Contains(string(found.Modifications), "maxFare")
	for _, internal := range []string{"processor", "Book {{route}}", "consentTemplate", "policy", "reuse",
		s.typeID} {
		s.NotContains(foundJSON, internal, "no processor, consent or policy configuration is published")
	}
}

func (s *TypeMetadataTestSuite) TestGetAnswersNotModifiedForAMatchingETag() {
	path := metadataPath + "/" + url.PathEscape(s.typeValue)
	status, headers, body := getPublic(s.T(), path, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var metadata typeMetadata
	s.Require().NoError(json.Unmarshal(body, &metadata))
	s.Equal(s.typeValue, metadata.Type)

	etag := headers.Get("ETag")
	s.Require().NotEmpty(etag)
	for _, header := range []string{etag, "W/" + etag, `"other", ` + etag} {
		status, _, body = getPublic(s.T(), path, map[string]string{"If-None-Match": header})
		s.Equal(http.StatusNotModified, status, header)
		s.Empty(body, header)
	}
}

func (s *TypeMetadataTestSuite) TestGetUnknownTypeIsNotFound() {
	status, _, body := getPublic(s.T(), metadataPath+"/"+url.PathEscape(uniqueTypeValue("unknown")), nil)
	s.Equal(http.StatusNotFound, status)
	s.Equal(codeMetadataTypeNotFound, errorCode(s.T(), body))
}

// getPublic sends an unauthenticated GET and returns the status, headers and body.
func getPublic(t *testing.T, path string, headers map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+path, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := testutils.GetNoRedirectHTTPClient().Do(req)
	if err != nil {
		t.Fatalf("sending GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	return resp.StatusCode, resp.Header, body
}
