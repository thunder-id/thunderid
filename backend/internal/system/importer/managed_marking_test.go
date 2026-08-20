/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import "testing"

func TestImportMarksOnlyWhatTheRequestDeclares(t *testing.T) {
	// A data plane uses this same API for its own work. Marking everything it wrote would make those
	// resources read only on the deployment that owns them.
	local := &ImportRequest{}
	if local.marksAsManaged("application", "app-1") {
		t.Fatal("a plain import must not claim ownership of what it writes")
	}
	if local.claimsControlPlaneAuthorship() {
		t.Fatal("a plain import must not be allowed to change control plane owned resources")
	}
}

func TestImportMarksTheResourcesTheRequestNames(t *testing.T) {
	req := &ImportRequest{ManagedResources: []ResourceRef{
		{ResourceType: "application", ID: "app-1"},
	}}

	if !req.marksAsManaged("application", "app-1") {
		t.Fatal("a named resource should be marked")
	}
	if req.marksAsManaged("application", "app-2") {
		t.Fatal("only the named resources are marked, so the rest stay editable here")
	}
	if !req.claimsControlPlaneAuthorship() {
		t.Fatal("a request naming managed resources writes on behalf of the control plane")
	}
}

func TestImportCanMarkAWholePayload(t *testing.T) {
	// A promotion sends configuration that all came from the control plane, so naming every resource
	// individually would be noise.
	req := &ImportRequest{Options: &ImportOptions{MarkManaged: true}}

	if !req.marksAsManaged("user", "user-1") || !req.marksAsManaged("flow", "flow-1") {
		t.Fatal("marking the request should cover every resource in it")
	}
	if !req.claimsControlPlaneAuthorship() {
		t.Fatal("a marked request writes on behalf of the control plane")
	}
}

func TestImportMatchesANamedResourceWithoutAType(t *testing.T) {
	// The type is optional, because an id is already unique and a caller should not have to know the
	// importer's name for a resource kind to hold it.
	req := &ImportRequest{ManagedResources: []ResourceRef{{ID: "app-1"}}}

	if !req.marksAsManaged("application", "app-1") {
		t.Fatal("an id alone should be enough to name a resource")
	}
}
