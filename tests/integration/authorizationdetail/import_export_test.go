// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"
)

const resourceTypeAuthorizationDetailType = "authorization_detail_type"

type ImportExportTestSuite struct {
	suite.Suite
}

func TestImportExportTestSuite(t *testing.T) {
	suite.Run(t, new(ImportExportTestSuite))
}

func (s *ImportExportTestSuite) TestExportedTypeImportsBack() {
	value := uniqueTypeValue("export")
	original := createType(s.T(), typeRequest{Type: value, Name: "Exported", Schema: flightSchema,
		ConsentTemplate: "Book {{route}}", Reuse: true})
	deleted := false
	defer func() {
		if !deleted {
			deleteType(s.T(), original.ID)
		}
	}()

	status, body := doJSON(s.T(), http.MethodPost, "/export",
		map[string]interface{}{"authorizationDetailTypes": []string{original.ID}})
	s.Require().Equal(http.StatusOK, status, string(body))
	var exported struct {
		Resources string `json:"resources"`
	}
	s.Require().NoError(json.Unmarshal(body, &exported))
	s.Contains(exported.Resources, "resource_type: "+resourceTypeAuthorizationDetailType)
	s.Contains(exported.Resources, value)

	// Importing into a deployment without the type creates it with its exported id.
	deleteType(s.T(), original.ID)
	deleted = true
	result := s.importDocument(exported.Resources, false, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("create", result["operation"], "%v", result)
	deleted = false

	status, body = doJSON(s.T(), http.MethodGet, typesPath+"/"+original.ID, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var imported typeResponse
	s.Require().NoError(json.Unmarshal(body, &imported))
	s.Equal(value, imported.Type)
	s.Equal("Book {{route}}", imported.ConsentTemplate)
	s.True(imported.Reuse)
	s.JSONEq(string(flightSchema), string(imported.Schema))

	// Without upsert, a dry run predicts the conflict the real import would hit.
	result = s.importDocument(exported.Resources, true, false)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeConflict, result["code"], "%v", result)

	// An import can delete the type.
	status, body = doJSON(s.T(), http.MethodPost, "/import", map[string]interface{}{
		"deletions": []map[string]string{{"resourceType": resourceTypeAuthorizationDetailType, "id": original.ID}},
		"options":   map[string]interface{}{"upsert": true, "continueOnError": true, "target": "runtime"},
	})
	s.Require().Equal(http.StatusOK, status, string(body))
	var deletion struct {
		Results []map[string]interface{} `json:"results"`
	}
	s.Require().NoError(json.Unmarshal(body, &deletion))
	s.Require().Len(deletion.Results, 1, string(body))
	s.Equal("success", deletion.Results[0]["status"], "%v", deletion.Results[0])
	status, _ = doJSON(s.T(), http.MethodGet, typesPath+"/"+original.ID, nil)
	s.Equal(http.StatusNotFound, status)
	deleted = true
}

// A processor credential is exported as a secret template variable, never in clear.
func (s *ImportExportTestSuite) TestExportKeepsProcessorCredentialsOutOfTheDocument() {
	const credential = "export-processor-token-value"
	created := createType(s.T(), typeRequest{Type: uniqueTypeValue("export-credential"), Name: "Exported Credential",
		Schema: flightSchema, Processor: &processorConfig{Endpoint: "https://processor.example.com/rar",
			Authentication: &processorAuthentication{Type: "bearer",
				Properties: map[string]string{"token": credential}}}})
	defer deleteType(s.T(), created.ID)

	status, body := doJSON(s.T(), http.MethodPost, "/export",
		map[string]interface{}{"authorizationDetailTypes": []string{created.ID}})
	s.Require().Equal(http.StatusOK, status, string(body))
	var exported struct {
		Resources            string `json:"resources"`
		EnvironmentVariables string `json:"environment_variables"`
	}
	s.Require().NoError(json.Unmarshal(body, &exported))

	s.NotContains(exported.Resources, credential, "the credential is not exported in clear")
	s.NotContains(exported.Resources, maskedSecret, "the credential is not exported masked")
	s.Regexp(`token: "?\{\{\.[A-Z0-9_]+\}\}"?`, exported.Resources,
		"the credential is a template variable")

	// The value travels only in the separate environment file, under the variable the document names.
	variable := regexp.MustCompile(`token: "?\{\{\.([A-Z0-9_]+)\}\}`).FindStringSubmatch(exported.Resources)
	s.Require().Len(variable, 2, exported.Resources)
	s.Contains(exported.EnvironmentVariables, variable[1]+"="+credential+"\n")
}

// importDocument imports the given content to the runtime and returns the outcome of its single document.
func (s *ImportExportTestSuite) importDocument(content string, dryRun, upsert bool) map[string]interface{} {
	status, body := doJSON(s.T(), http.MethodPost, "/import", map[string]interface{}{
		"content": content,
		"dryRun":  dryRun,
		"options": map[string]interface{}{"upsert": upsert, "continueOnError": true, "target": "runtime"},
	})
	s.Require().Equal(http.StatusOK, status, string(body))
	var resp struct {
		Results []map[string]interface{} `json:"results"`
	}
	s.Require().NoError(json.Unmarshal(body, &resp))
	s.Require().Len(resp.Results, 1, string(body))
	return resp.Results[0]
}
