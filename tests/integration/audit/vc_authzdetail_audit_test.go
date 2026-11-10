// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// auditVCClaim is a claim the verifiable credential resources of these tests name. No line may carry
	// it.
	auditVCClaim = "audit_vc_claim_marker"
	// auditVCType is the credential type those resources name. No line may carry it.
	auditVCType = "https://audit-vc.test/AuditCredential"
	// auditAuthzDetailName names an authorization details type. No line may carry it.
	auditAuthzDetailName = "Audit Authorization Detail Marker"

	// declaredCredentialConfigurationID and declaredPresentationDefinitionID are resources the integration
	// deployment declares in files, which the management API refuses to change.
	declaredCredentialConfigurationID = "decl-credential-config-1"
	declaredPresentationDefinitionID  = "decl-presentation-def-1"
)

// vcResource is a verifiable credential resource the management API serves.
type vcResource struct {
	kind       string
	path       string
	targetType string
	declaredID string
	// body is the request body of a resource with handle in ouID.
	body func(handle, ouID string) map[string]any
	// notFound, immutable and invalid are the reasons of an update of a resource that does not exist, of a
	// change to a declared one and of a request without a credential type.
	notFound, immutable, invalid string
}

// vcResources returns the presentation definition and the credential configuration resources.
func vcResources() []vcResource {
	return []vcResource{
		{
			kind: "presentation definition", path: "/openid4vp/presentation-definitions",
			targetType: "presentation_definition", declaredID: declaredPresentationDefinitionID,
			body: func(handle, ouID string) map[string]any {
				return map[string]any{"handle": handle, "ouId": ouID, "name": "Audit Presentation " + handle,
					"vct": auditVCType, "format": "dc+sd-jwt", "requestedClaims": []string{auditVCClaim}}
			},
			notFound: "vp_2002", immutable: "vp_2005", invalid: "vp_2001",
		},
		{
			kind: "credential configuration", path: "/openid4vci/credential-configurations",
			targetType: "credential_configuration", declaredID: declaredCredentialConfigurationID,
			body: func(handle, ouID string) map[string]any {
				return map[string]any{"handle": handle, "ouId": ouID, "name": "Audit Credential " + handle,
					"vct": auditVCType, "format": "dc+sd-jwt",
					"claims": []map[string]any{{"name": auditVCClaim, "displayName": "Audit Claim"}}}
			},
			notFound: "vci_2002", immutable: "vci_2005", invalid: "vci_2001",
		},
	}
}

// Creating, updating and deleting a presentation definition and a credential configuration writes one line
// each, with the admin as actor and the resource as target and subject. A delete of one that does not
// exist still succeeds and writes no line. No handle, name, credential type or claim appears in any line.
func (ts *AuditLogTestSuite) TestAVerifiableCredentialResourceLifecycleWritesOneLinePerChange() {
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	for _, resource := range vcResources() {
		ts.Run(resource.kind, func() {
			ouID := ts.createOrganizationUnit("vc-" + resource.targetType)
			handle := ts.prefix + "vc-life"
			created, id := ts.create(resource.path, resource.body(handle, ouID))
			updated := ts.put(resource.path+"/"+id, resource.body(handle+"-renamed", ouID))
			deleted := ts.deleted(resource.path + "/" + id)
			deletedAgain := ts.deleted(resource.path + "/" + id)

			written := ts.linesPerCall(created, updated, deleted, deletedAgain)
			ts.Empty(written[3], "a delete that removed nothing writes no line")
			resourceRef := testutils.AuditRef{Type: resource.targetType, ID: id}
			for i, action := range []string{"create", "update", "delete"} {
				event := resource.targetType + "." + action
				ts.Require().Len(written[i], 1, event)
				line := written[i][0]
				ts.Equal(lineKeys, ts.keys(line.Raw), event)
				ts.Equal(event, line.Event)
				ts.Equal(action, line.Action, event)
				ts.Equal(resourceRef, line.Target, event)
				ts.Equal(resourceRef, line.Subject, event)
				ts.Equal(adminRef, line.Actor, event)
				ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome, event)
				ts.Nil(line.Client, "%s: a loopback peer is not recorded", event)
				ts.requireNothingOfTheVCResource(line, handle, ouID)
			}
		})
	}
}

// Every refused change of a presentation definition or a credential configuration is recorded: a failed
// attempt with the ID the caller sent, or none on a create, and no subject; an attempt by a manager of
// another unit as denied, with the resource as subject when it exists.
func (ts *AuditLogTestSuite) TestEveryRefusedVerifiableCredentialResourceChangeIsRecorded() {
	siblingOU := ts.createOrganizationUnit("vc-paths")
	manager := ts.createScopedManager("vc-paths-mgr", nil, []string{"org"})
	missing := unusedID()
	handle := ts.prefix + "vc-paths"

	type vcCase struct {
		name    string
		client  *http.Client
		method  string
		path    string
		body    map[string]any
		status  int
		event   string
		target  string
		subject bool
		outcome map[string]string
	}
	var cases []vcCase
	for _, resource := range vcResources() {
		_, existing := ts.create(resource.path, resource.body(handle, siblingOU))
		failed := func(reason string) map[string]string {
			return map[string]string{"result": "failed", "reason": reason}
		}
		denied := map[string]string{"result": "denied", "reason": "forbidden"}
		withoutType := resource.body(handle+"-untyped", siblingOU)
		delete(withoutType, "vct")
		cases = append(cases,
			vcCase{name: resource.kind + ": a create without a credential type", client: testutils.GetHTTPClient(),
				method: http.MethodPost, path: resource.path, body: withoutType, status: http.StatusBadRequest,
				event: resource.targetType + ".create", outcome: failed(resource.invalid)},
			vcCase{name: resource.kind + ": an update of one that does not exist", client: testutils.GetHTTPClient(),
				method: http.MethodPut, path: resource.path + "/" + missing, body: resource.body(handle, siblingOU),
				status: http.StatusNotFound, event: resource.targetType + ".update", target: missing,
				outcome: failed(resource.notFound)},
			vcCase{name: resource.kind + ": a delete of one declared in a file", client: testutils.GetHTTPClient(),
				method: http.MethodDelete, path: resource.path + "/" + resource.declaredID,
				status: http.StatusConflict, event: resource.targetType + ".delete", target: resource.declaredID,
				outcome: failed(resource.immutable)},
			vcCase{name: resource.kind + ": a create in a unit the caller does not manage", client: manager.client,
				method: http.MethodPost, path: resource.path, body: resource.body(handle+"-denied", siblingOU),
				status: http.StatusForbidden, event: resource.targetType + ".create", outcome: denied},
			vcCase{name: resource.kind + ": an update in a unit the caller does not manage", client: manager.client,
				method: http.MethodPut, path: resource.path + "/" + existing, body: resource.body(handle, siblingOU),
				status: http.StatusForbidden, event: resource.targetType + ".update", target: existing,
				subject: true, outcome: denied},
			vcCase{name: resource.kind + ": a delete in a unit the caller does not manage", client: manager.client,
				method: http.MethodDelete, path: resource.path + "/" + existing, status: http.StatusForbidden,
				event: resource.targetType + ".delete", target: existing, subject: true, outcome: denied},
		)
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		body := ""
		if tc.body != nil {
			body = ts.jsonBody(tc.body)
		}
		response, read := ts.exchange(tc.client, tc.method, tc.path, body)
		ts.Require().Equal(tc.status, response.StatusCode, "%s: %s", tc.name, string(read))
		responses[i] = response
	}
	lines := ts.linesOf(responses...)
	for i, tc := range cases {
		line := lines[i]
		targetType := tc.event[:strings.LastIndex(tc.event, ".")]
		ts.Equal(tc.event, line.Event, tc.name)
		ts.Equal(testutils.AuditRef{Type: targetType, ID: tc.target}, line.Target, tc.name)
		if tc.subject {
			ts.Equal(line.Target, line.Subject, "%s: a refusal names the resource it found", tc.name)
		} else {
			ts.Equal(testutils.AuditRef{}, line.Subject, "%s: the attempt names no resource", tc.name)
		}
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		if tc.client == manager.client {
			ts.Equal(testutils.AuditRef{Type: "user", ID: manager.id}, line.Actor, tc.name)
		}
		ts.requireNothingOfTheVCResource(line, handle, siblingOU)
	}
}

// An import with upsert creates a presentation definition and a credential configuration whose ID nothing
// has, and updates the one that has it. Each writes the resource's own line before the import's, and the
// create is not preceded by a failed update: the import looks the resource up rather than attempting an
// update first.
func (ts *AuditLogTestSuite) TestAnImportWritesTheEventOfTheVerifiableCredentialResourceItWrites() {
	ouID := ts.createOrganizationUnit("vc-import")
	succeeded := map[string]string{"result": "succeeded"}
	for _, resource := range vcResources() {
		ts.Run(resource.kind, func() {
			id := unusedID()
			handle := ts.prefix + "vc-imp"
			ts.T().Cleanup(func() { ts.send(http.MethodDelete, resource.path+"/"+id, "") })
			document := resource.body(handle, ouID)
			document["resource_type"] = resource.targetType
			document["id"] = id
			content, err := json.Marshal(document)
			ts.Require().NoError(err)

			resourceRef := testutils.AuditRef{Type: resource.targetType, ID: id}
			for _, operation := range []string{"create", "update"} {
				imported, result := ts.importAs(testutils.GetHTTPClient(), string(content))
				ts.Require().Equal("success", result.Status, result.Message)
				ts.Require().Equal(operation, result.Operation, result.Message)
				line := ts.resourceLineOfImport(imported, succeeded)
				ts.Equal(resource.targetType+"."+operation, line.Event)
				ts.Equal(resourceRef, line.Target, operation)
				ts.Equal(resourceRef, line.Subject, operation)
				ts.Equal(succeeded, line.Outcome, operation)
				ts.requireNothingOfTheVCResource(line, handle, ouID)
			}

			// A dry run finds the resource it would update and writes nothing but the import's own line.
			request, err := json.Marshal(testutils.ImportRequest{Content: string(content), DryRun: true,
				Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"}})
			ts.Require().NoError(err)
			dryRun, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/import", string(request))
			ts.Require().Equal(http.StatusOK, dryRun.StatusCode, string(body))
			var result testutils.ImportResponse
			ts.Require().NoError(json.Unmarshal(body, &result))
			ts.Require().Len(result.Results, 1, string(body))
			ts.Equal("update", result.Results[0].Operation, "a dry run reports the update it would make")
			written := ts.linesPerCall(dryRun)[0]
			ts.Require().Len(written, 1, "a dry run writes only the import's line")
			ts.Equal(importEvent, written[0].Event)
			ts.Equal(map[string]string{"dry_run": "true"}, ts.details(written[0]))
		})
	}
}

// A data plane takes an authorization details type only through an import with the management API key: a
// write with the admin's token reaches the service, which refuses it as denied, whether it came through the
// management API or an import. The refusal comes before the type is looked up, so an update or delete of an
// ID no type has keeps that ID as target only: no denied line names a subject, so a caller cannot make the
// log name a type it made up.
func (ts *AuditLogTestSuite) TestAnAuthorizationDetailTypeWriteWithATokenIsRecordedAsDenied() {
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	missing := unusedID()
	typeValue := "https://audit-authz.test/" + ts.prefix + "denied"
	body := ts.jsonBody(authzDetailTypeBody(typeValue, auditAuthzDetailName))

	created := ts.gatewayCall(http.MethodPost, "/authorization-detail-types", body, http.StatusForbidden)
	updated := ts.gatewayCall(http.MethodPut, "/authorization-detail-types/"+missing, body, http.StatusForbidden)
	deleted := ts.gatewayCall(http.MethodDelete, "/authorization-detail-types/"+missing, "", http.StatusForbidden)
	imported, result := ts.importAs(testutils.GetHTTPClient(), authzDetailTypeDocument("", typeValue,
		auditAuthzDetailName))
	ts.Require().Equal("failed", result.Status, "the admin's token imports no type on a data plane")

	written := ts.linesPerCall(created, updated, deleted, imported)
	ts.Require().Len(written[3], 2, "the import wrote the refused create's line and its own")
	ts.Equal(importEvent, written[3][1].Event)
	ts.Equal(map[string]string{"result": "failed", "reason": "item_failed"}, written[3][1].Outcome)
	denied := map[string]string{"result": "denied", "reason": "forbidden"}
	missingRef := testutils.AuditRef{Type: "authorization_detail_type", ID: missing}
	for i, want := range []struct {
		event  string
		target testutils.AuditRef
	}{
		{"authorization_detail_type.create", testutils.AuditRef{Type: "authorization_detail_type"}},
		{"authorization_detail_type.update", missingRef},
		{"authorization_detail_type.delete", missingRef},
		{"authorization_detail_type.create", testutils.AuditRef{Type: "authorization_detail_type"}},
	} {
		ts.Require().NotEmpty(written[i], "call %d wrote no line", i)
		line := written[i][0]
		if i < 3 {
			ts.Require().Len(written[i], 1, want.event)
		}
		ts.Equal(want.event, line.Event, "line %d", i)
		ts.Equal(want.target, line.Target, "line %d", i)
		ts.Equal(testutils.AuditRef{}, line.Subject, "line %d: a denied write names no subject", i)
		ts.Equal(adminRef, line.Actor, "line %d", i)
		ts.Equal(denied, line.Outcome, "line %d", i)
		ts.NotContains(line.Raw, typeValue, "line %d: the type value is not recorded", i)
		ts.NotContains(line.Raw, auditAuthzDetailName, "line %d: the type's name is not recorded", i)
	}
}

// An import with the management API key creates an authorization details type whose ID no type has,
// updates the one that has it and deletes it, each writing the type's own line before the import's, with
// the deployment's system actor. The create is not preceded by a failed update, and a deletion of a type
// that is gone writes only the import's line. A refused create or update records the reason and no
// subject. Neither the type's value nor its name appears in any line.
func (ts *AuditLogTestSuite) TestAnAuthorizationDetailTypeImportRecordsItsOutcome() {
	systemRef := testutils.AuditRef{Type: "system", ID: "management-api-key"}
	typeID := unusedID()
	typeValue := "https://audit-authz.test/" + ts.prefix + "import"
	ts.T().Cleanup(func() { _ = testutils.DeleteImportedResource("authorization_detail_type", typeID) })
	keyClient := testutils.GetManagementAPIKeyHTTPClient()
	document := authzDetailTypeDocument(typeID, typeValue, auditAuthzDetailName)

	createdByImport, created := ts.importAs(keyClient, document)
	ts.Require().Equal("create", created.Operation, created.Message)
	updatedByImport, updated := ts.importAs(keyClient, document)
	ts.Require().Equal("update", updated.Operation, updated.Message)
	refusedUpdate, failedUpdate := ts.importAs(keyClient, authzDetailTypeDocument(typeID, typeValue, ""))
	ts.Require().Equal("failed", failedUpdate.Status)
	refusedCreate, failedCreate := ts.importAs(keyClient, authzDetailTypeDocument(unusedID(),
		typeValue+"-unnamed", ""))
	ts.Require().Equal("failed", failedCreate.Status)
	deletedByImport := ts.deleteByImport(keyClient, "authorization_detail_type", typeID)
	deletedAgain := ts.deleteByImport(keyClient, "authorization_detail_type", typeID)

	written := ts.linesPerCall(createdByImport, updatedByImport, refusedUpdate, refusedCreate, deletedByImport,
		deletedAgain)
	ts.Require().Len(written[5], 1, "a deletion of a type that is gone writes only the import's line")
	ts.Equal(importEvent, written[5][0].Event)
	typeRef := testutils.AuditRef{Type: "authorization_detail_type", ID: typeID}
	succeeded := map[string]string{"result": "succeeded"}
	invalidName := map[string]string{"result": "failed", "reason": "rar_1005"}
	for i, want := range []struct {
		event   string
		target  testutils.AuditRef
		subject testutils.AuditRef
		outcome map[string]string
	}{
		{"authorization_detail_type.create", typeRef, typeRef, succeeded},
		{"authorization_detail_type.update", typeRef, typeRef, succeeded},
		{"authorization_detail_type.update", typeRef, testutils.AuditRef{}, invalidName},
		{"authorization_detail_type.create", testutils.AuditRef{Type: "authorization_detail_type"},
			testutils.AuditRef{}, invalidName},
		{"authorization_detail_type.delete", typeRef, typeRef, succeeded},
	} {
		ts.Require().Len(written[i], 2, "%s: the type's line and the import's", want.event)
		ts.Equal(importEvent, written[i][1].Event, want.event)
		line := written[i][0]
		ts.Equal(want.event, line.Event, "line %d", i)
		ts.Equal(want.target, line.Target, "line %d", i)
		ts.Equal(want.subject, line.Subject, "line %d", i)
		ts.Equal(systemRef, line.Actor, "line %d", i)
		ts.Equal(want.outcome, line.Outcome, "line %d", i)
		ts.NotContains(line.Raw, typeValue, "line %d: the type value is not recorded", i)
		ts.NotContains(line.Raw, auditAuthzDetailName, "line %d: the type's name is not recorded", i)
		ts.NotContains(line.Raw, managementAPIKey, "line %d: the key is never recorded", i)
	}
}

// requireNothingOfTheVCResource checks that line carries no handle, name, credential type or claim of a
// verifiable credential resource, nor the unit it belongs to.
func (ts *AuditLogTestSuite) requireNothingOfTheVCResource(line testutils.AuditLine, handle, ouID string) {
	ts.T().Helper()
	ts.NotContains(line.Raw, handle, "%s: the handle is not recorded", line.Event)
	ts.NotContains(line.Raw, ts.prefix, "%s: no handle or name is recorded", line.Event)
	ts.NotContains(line.Raw, auditVCType, "%s: the credential type is not recorded", line.Event)
	ts.NotContains(line.Raw, "audit-vc.test", "%s: no part of the credential type is recorded", line.Event)
	ts.NotContains(line.Raw, auditVCClaim, "%s: no claim is recorded", line.Event)
	ts.NotContains(line.Raw, ouID, "%s: the organization unit is not recorded", line.Event)
}

// deleteByImport asks an import, as client, to delete the resource of resourceType with id, and returns the
// response.
func (ts *AuditLogTestSuite) deleteByImport(client *http.Client, resourceType, id string) *http.Response {
	ts.T().Helper()
	request, err := json.Marshal(testutils.ImportRequest{
		Options:   testutils.ImportOptions{Target: "runtime"},
		Deletions: []testutils.ResourceDeletion{{ResourceType: resourceType, ID: id}},
	})
	ts.Require().NoError(err)
	deleted, body := ts.exchange(client, http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, deleted.StatusCode, string(body))
	return deleted
}

// authzDetailTypeBody is the request body of an authorization details type with typeValue and name.
func authzDetailTypeBody(typeValue, name string) map[string]any {
	return map[string]any{
		"type": typeValue, "name": name,
		"schema": map[string]any{"type": "object", "required": []string{"route"},
			"properties": map[string]any{"route": map[string]any{"type": "string"}}},
	}
}

// authzDetailTypeDocument is the import document of an authorization details type with id, typeValue and
// name. An empty id leaves the ID to the server.
func authzDetailTypeDocument(id, typeValue, name string) string {
	document := authzDetailTypeBody(typeValue, name)
	document["resource_type"] = "authorization_detail_type"
	if id != "" {
		document["id"] = id
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		panic(fmt.Sprintf("encoding an authorization details type document: %v", err))
	}
	return string(encoded)
}

// importAs imports content to the runtime with upsert on, as client, and returns the response and the one
// result of the import.
func (ts *AuditLogTestSuite) importAs(client *http.Client, content string) (*http.Response, testutils.ImportItem) {
	ts.T().Helper()
	request, err := json.Marshal(testutils.ImportRequest{
		Content: content,
		Options: testutils.ImportOptions{Upsert: true, ContinueOnError: true, Target: "runtime"},
	})
	ts.Require().NoError(err)
	imported, body := ts.exchange(client, http.MethodPost, "/import", string(request))
	ts.Require().Equal(http.StatusOK, imported.StatusCode, string(body))
	var result testutils.ImportResponse
	ts.Require().NoError(json.Unmarshal(body, &result))
	ts.Require().Len(result.Results, 1, string(body))
	return imported, result.Results[0]
}
