// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authorizationdetail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	resourceTypeAuthorizationDetailType = "authorization_detail_type"

	// codeTypeIDConflict is returned when a type is created with an id another type has.
	codeTypeIDConflict = "RAR-1015"
	// codeImportInvalidYAML is returned for an import document that cannot be read.
	codeImportInvalidYAML = "IMP-1002"
	// codeImportUnresolvedReference is returned for a reference to a value the deployment does not hold.
	codeImportUnresolvedReference = "IMP-1006"
	// codeImportMisplacedSecret is returned for a secret referred to from a field that is not a credential.
	codeImportMisplacedSecret = "IMP-1007"
)

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

// A processor credential exported as a template variable imports back when its value is supplied, and the
// stored credential is the one that was exported.
func (s *ImportExportTestSuite) TestProcessorCredentialImportsBackWithItsValue() {
	const credential = "reimport-processor-token-value"
	value := uniqueTypeValue("reimport-credential")
	original := createType(s.T(), typeRequest{Type: value, Name: "Reimported Credential", Schema: flightSchema,
		Processor: &processorConfig{Endpoint: "https://processor.example.com/rar",
			Authentication: &processorAuthentication{Type: "bearer",
				Properties: map[string]string{"token": credential}}}})
	deleted := false
	defer func() {
		if !deleted {
			deleteType(s.T(), original.ID)
		}
	}()

	resources, environment := s.export(original.ID)
	variable := regexp.MustCompile(`token: "?\{\{\.([A-Z0-9_]+)\}\}`).FindStringSubmatch(resources)
	s.Require().Len(variable, 2, resources)
	variables := map[string]interface{}{variable[1]: credential}

	// While the type exists, an upsert updates it, and a dry run predicts that.
	result := s.importDocumentWithVariables(resources, variables, true, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("update", result["operation"], "%v", result)
	result = s.importDocumentWithVariables(resources, variables, false, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("update", result["operation"], "%v", result)

	deleteType(s.T(), original.ID)
	deleted = true
	result = s.importDocumentWithVariables(resources, variables, false, false)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("create", result["operation"], "%v", result)
	deleted = false

	status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+original.ID, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var imported typeResponse
	s.Require().NoError(json.Unmarshal(body, &imported))
	s.Require().NotNil(imported.Processor)
	s.Require().NotNil(imported.Processor.Authentication)
	s.Equal("bearer", imported.Processor.Authentication.Type)
	s.Equal(maskedSecret, imported.Processor.Authentication.Properties["token"])

	_, environment = s.export(original.ID)
	s.Contains(environment, variable[1]+"="+credential+"\n", "the imported credential is the exported one")
}

// A dry run predicts what the import would do without writing anything: a new type is created, an existing
// one is updated with upsert, and each refusal the real import would hit is reported.
func (s *ImportExportTestSuite) TestDryRunPredictsTheOutcome() {
	value := uniqueTypeValue("dry-run")
	existing := createType(s.T(), typeRequest{Type: value, Name: "Dry Run", Schema: flightSchema})
	defer deleteType(s.T(), existing.ID)

	newID := uniqueTypeID()
	newValue := uniqueTypeValue("dry-run-new")
	result := s.importDocument(typeDocument(newID, newValue, ""), true, false)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("create", result["operation"], "%v", result)
	status, _ := doJSON(s.T(), http.MethodGet, typesPath+"/"+newID, nil)
	s.Equal(http.StatusNotFound, status, "a dry run writes nothing")

	result = s.importDocument(typeDocument(existing.ID, value, ""), true, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("update", result["operation"], "%v", result)

	// The type value of an existing type cannot change, in a dry run or for real.
	result = s.importDocument(typeDocument(existing.ID, newValue, ""), true, true)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeValueImmutable, result["code"], "%v", result)
	result = s.importDocument(typeDocument(existing.ID, newValue, ""), false, true)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeValueImmutable, result["code"], "%v", result)

	// Without upsert, a document reusing an existing id is an id conflict.
	result = s.importDocument(typeDocument(existing.ID, newValue, ""), true, false)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeIDConflict, result["code"], "%v", result)
	result = s.importDocument(typeDocument(existing.ID, newValue, ""), false, false)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeIDConflict, result["code"], "%v", result)

	// A new id with a registered type value is a type conflict.
	result = s.importDocument(typeDocument(newID, value, ""), true, false)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeTypeConflict, result["code"], "%v", result)

	// A schema the registry refuses is refused on import.
	invalid := "schema:\n  type: object\n  properties:\n    actions:\n      type: array\n"
	for _, upsert := range []bool{false, true} {
		for _, dryRun := range []bool{true, false} {
			result = s.importDocument(typeDocument(newID, newValue, invalid), dryRun, upsert)
			s.Equal("failed", result["status"], "dry run %v, upsert %v: %v", dryRun, upsert, result)
			s.Equal(codeInvalidSchema, result["code"], "dry run %v, upsert %v: %v", dryRun, upsert, result)
		}
	}
	result = s.importDocument(typeDocument(existing.ID, value, invalid), true, true)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeInvalidSchema, result["code"], "%v", result)
	result = s.importDocument(typeDocument(newID, newValue, "schema: null\n"), true, false)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeInvalidSchema, result["code"], "a type needs a schema: %v", result)

	// A schema written in YAML may repeat a member through an alias.
	aliased := "schema:\n  type: object\n  properties:\n    from: &place\n      type: string\n" +
		"    to: *place\n    route:\n      type: string\n"
	result = s.importDocument(typeDocument(newID, newValue, aliased), true, false)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("create", result["operation"], "%v", result)
	status, _ = doJSON(s.T(), http.MethodGet, typesPath+"/"+newID, nil)
	s.Equal(http.StatusNotFound, status, "a refused import writes nothing")
}

// A dry run of an upsert that moves a type's processor to another endpoint predicts that the credential must
// be given again.
func (s *ImportExportTestSuite) TestDryRunPredictsAProcessorCredentialIsRequiredAgain() {
	value := uniqueTypeValue("dry-run-credential")
	existing := createType(s.T(), typeRequest{Type: value, Name: "Dry Run", Schema: flightSchema,
		Processor: &processorConfig{Endpoint: "https://processor.example.com/rar",
			Authentication: &processorAuthentication{Type: "bearer",
				Properties: map[string]string{"token": "dry-run-token"}}}})
	defer deleteType(s.T(), existing.ID)

	moved := "processor:\n  endpoint: https://elsewhere.example.com/rar\n"
	result := s.importDocument(typeDocument(existing.ID, value, moved), true, true)
	s.Equal("failed", result["status"], "%v", result)
	s.Equal(codeProcessorAuthenticationRequired, result["code"], "%v", result)

	// The same endpoint keeps the stored credential.
	kept := "processor:\n  endpoint: https://processor.example.com/rar\n"
	result = s.importDocument(typeDocument(existing.ID, value, kept), true, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("update", result["operation"], "%v", result)
}

// A document that cannot be read as a type, or that refers to a secret where no credential belongs, is
// refused without writing anything.
func (s *ImportExportTestSuite) TestMalformedDocumentsAreRefused() {
	id := uniqueTypeID()
	value := uniqueTypeValue("malformed-document")
	cases := map[string]struct {
		document string
		code     string
	}{
		"a schema that is not JSON": {typeDocument(id, value, "schema: '{\"type\":'\n"), codeImportInvalidYAML},
		"an id over 36 characters":  {typeDocument(strings.Repeat("i", 37), value, ""), codeImportInvalidYAML},
		"a description that is a list": {
			typeDocument(id, value, "") + "description: [a, b]\n", codeImportInvalidYAML},
		"an unknown processor authentication": {typeDocument(id, value,
			"processor:\n  endpoint: https://processor.example.com/rar\n  authentication:\n"+
				"    type: kerberos\n"), codeImportInvalidYAML},
		"a secret as the basic username": {typeDocument(id, value,
			"processor:\n  endpoint: https://processor.example.com/rar\n  authentication:\n"+
				"    type: basic\n    properties:\n      username: sec:ADT_PROCESSOR_USER\n"+
				"      password: processor-password\n"), codeImportMisplacedSecret},
		"a secret the deployment does not hold": {typeDocument(id, value,
			"processor:\n  endpoint: https://processor.example.com/rar\n  authentication:\n"+
				"    type: bearer\n    properties:\n      token: sec:ADT_PROCESSOR_TOKEN_NOT_HELD\n"),
			codeImportUnresolvedReference},
	}
	for name, tc := range cases {
		for _, dryRun := range []bool{true, false} {
			result := s.importDocument(tc.document, dryRun, true)
			s.Equal("failed", result["status"], "%s (dry run %v): %v", name, dryRun, result)
			s.Equal(tc.code, result["code"], "%s (dry run %v): %v", name, dryRun, result)
		}
	}
	status, _ := doJSON(s.T(), http.MethodGet, typesPath+"/"+id, nil)
	s.Equal(http.StatusNotFound, status, "a refused document writes nothing")
}

// A processor that authenticates nothing exports authentication none, so importing it clears a credential the
// deployment stores, while a document without the block keeps it.
func (s *ImportExportTestSuite) TestExportedNoneAuthenticationClearsTheStoredCredential() {
	endpoint := "https://processor.example.com/rar"
	req := typeRequest{Type: uniqueTypeValue("export-none"), Name: "Exported None", Schema: flightSchema,
		Processor: &processorConfig{Endpoint: endpoint}}
	created := createType(s.T(), req)
	defer deleteType(s.T(), created.ID)

	resources, _ := s.export(created.ID)
	s.Regexp(`authentication:\s+type: none`, resources, "authenticating nothing is exported, not left out")

	storeBearer := func() {
		req.Processor = &processorConfig{Endpoint: endpoint, Authentication: &processorAuthentication{
			Type: "bearer", Properties: map[string]string{"token": "stored-token"}}}
		updated := updateType(s.T(), created.ID, req)
		s.Require().NotNil(updated.Processor.Authentication)
		s.Require().Equal("bearer", updated.Processor.Authentication.Type)
	}
	storedAuthenticationType := func() string {
		status, body := doJSON(s.T(), http.MethodGet, typesPath+"/"+created.ID, nil)
		s.Require().Equal(http.StatusOK, status, string(body))
		var stored typeResponse
		s.Require().NoError(json.Unmarshal(body, &stored))
		s.Require().NotNil(stored.Processor)
		s.Require().NotNil(stored.Processor.Authentication)
		return stored.Processor.Authentication.Type
	}

	storeBearer()
	result := s.importDocument(resources, false, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("none", storedAuthenticationType(), "the imported none clears the stored credential")

	storeBearer()
	withoutBlock := regexp.MustCompile(`\n\s+authentication:\n\s+type: none`).ReplaceAllString(resources, "")
	s.Require().NotContains(withoutBlock, "authentication:")
	result = s.importDocument(withoutBlock, false, true)
	s.Equal("success", result["status"], "%v", result)
	s.Equal("bearer", storedAuthenticationType(), "a document without the block keeps the stored credential")
}

// Exporting every type exports the registered ones and leaves out those declared in files, which a deployment
// already has.
func (s *ImportExportTestSuite) TestExportOfAllTypesLeavesOutDeclaredTypes() {
	value := uniqueTypeValue("export-all")
	registered := createType(s.T(), typeRequest{Type: value, Name: "Exported With All", Schema: flightSchema})
	defer deleteType(s.T(), registered.ID)

	status, body := doJSON(s.T(), http.MethodPost, "/export",
		map[string]interface{}{"authorizationDetailTypes": []string{"*"}})
	s.Require().Equal(http.StatusOK, status, string(body))
	var exported struct {
		Resources string `json:"resources"`
	}
	s.Require().NoError(json.Unmarshal(body, &exported))
	s.Contains(exported.Resources, "id: "+registered.ID)
	s.Contains(exported.Resources, value)
	s.NotContains(exported.Resources, declarativeTypeID, "a declared type is not exported")
}

// export exports a type and returns the resources document and the environment variables file.
func (s *ImportExportTestSuite) export(id string) (string, string) {
	status, body := doJSON(s.T(), http.MethodPost, "/export",
		map[string]interface{}{"authorizationDetailTypes": []string{id}})
	s.Require().Equal(http.StatusOK, status, string(body))
	var exported struct {
		Resources            string `json:"resources"`
		EnvironmentVariables string `json:"environment_variables"`
	}
	s.Require().NoError(json.Unmarshal(body, &exported))
	return exported.Resources, exported.EnvironmentVariables
}

// typeDocument returns a type's import document with the flight schema written as YAML, followed by the given
// extra YAML, which replaces the schema when it declares one.
func typeDocument(id, value, extra string) string {
	document := "resource_type: " + resourceTypeAuthorizationDetailType + "\nid: " + id + "\ntype: " + value +
		"\nname: Imported Type\nconsentTemplate: Book {{route}}\nreuse: true\n"
	if !strings.Contains(extra, "schema:") {
		document += "schema:\n  type: object\n  required: [route]\n  properties:\n" +
			"    route:\n      type: string\n      title: Route\n" +
			"    maxFare:\n      type: number\n      minimum: 0\n      maximum: 5000\n" +
			"      policy:\n        modification: decrease\n        modifiableBy: [user, client]\n"
	}
	return document + extra
}

// uniqueTypeID returns a type id no other test run has used, within the 36 characters an id may have.
func uniqueTypeID() string {
	return fmt.Sprintf("adt-import-%d", time.Now().UnixNano())
}

// importDocument imports the given content to the runtime and returns the outcome of its single document.
func (s *ImportExportTestSuite) importDocument(content string, dryRun, upsert bool) map[string]interface{} {
	return s.importDocumentWithVariables(content, nil, dryRun, upsert)
}

// importDocumentWithVariables imports the given content with the values of its template variables, and
// returns the outcome of its single document.
func (s *ImportExportTestSuite) importDocumentWithVariables(content string, variables map[string]interface{},
	dryRun, upsert bool) map[string]interface{} {
	status, body := doJSON(s.T(), http.MethodPost, "/import", map[string]interface{}{
		"content":   content,
		"variables": variables,
		"dryRun":    dryRun,
		"options":   map[string]interface{}{"upsert": upsert, "continueOnError": true, "target": "runtime"},
	})
	s.Require().Equal(http.StatusOK, status, string(body))
	var resp struct {
		Results []map[string]interface{} `json:"results"`
	}
	s.Require().NoError(json.Unmarshal(body, &resp))
	s.Require().Len(resp.Results, 1, string(body))
	return resp.Results[0]
}
