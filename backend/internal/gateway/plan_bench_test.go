// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/export"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// benchResources is how many resources the benchmarked bundle holds.
const benchResources = 3000

// benchBundle writes a bundle of organization units, flows and applications, the applications naming a
// unit and a flow by id and referring to a variable and a secret, as an export of a large deployment
// would. Revision changes the description of one in twenty applications, adds and drops a few.
func benchBundle(revision int) *export.ExportResponse {
	units, flows := benchResources/10, benchResources/10
	apps := benchResources - units - flows
	response := &export.ExportResponse{}
	add := func(content string) {
		response.Files = append(response.Files, export.ExportFile{Content: content})
	}
	for i := 0; i < units; i++ {
		add(fmt.Sprintf("resource_type: organization_unit\nid: ou-%04d\nname: unit %d\nhandle: unit-%d\n"+
			"description: A unit\nparent: ou-%04d", i, i, i, i/10))
	}
	for i := 0; i < flows; i++ {
		add(fmt.Sprintf("resource_type: flow\nid: flow-%04d\nname: flow %d\nhandle: flow-%d\n"+
			"flowType: AUTHENTICATION\n"+
			"nodes:\n  - id: start\n    type: START\n    onSuccess: prompt\n  - id: prompt\n    type: PROMPT\n"+
			"    prompts:\n      - inputs:\n          - identifier: username\n            type: TEXT_INPUT\n"+
			"            required: true\n    onSuccess: end\n  - id: end\n    type: END", i, i, i))
	}
	for i := 0; i < apps; i++ {
		if revision > 0 && i%97 == 0 {
			continue
		}
		description := "An application"
		if revision > 0 && i%20 == 0 {
			description = fmt.Sprintf("An application, revision %d", revision)
		}
		add(benchApplication(i, description))
	}
	if revision > 0 {
		for i := apps; i < apps+25; i++ {
			add(benchApplication(i, "A new application"))
		}
	}
	return response
}

func benchApplication(i int, description string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "resource_type: application\nid: app-%05d\nname: app %d\ndescription: %s\n", i, i, description)
	if i%2 == 0 {
		b.WriteString("projectId: shop\n")
	}
	fmt.Fprintf(&b, "ouId: ou-%04d\nauthFlowId: flow-%04d\nregistrationFlowId: flow-%04d\n",
		i%(benchResources/10), i%(benchResources/10), (i+1)%(benchResources/10))
	b.WriteString("isRegistrationFlowEnabled: true\nallowedUserTypes:\n  - customer\n")
	fmt.Fprintf(&b, "url: var:APP_%05d_URL\nlogoUrl: https://example.test/logo.png\n", i)
	b.WriteString("assertion:\n  validityPeriod: 3600\n  userAttributes:\n    - email\n    - given_name\n")
	b.WriteString("inboundAuthConfig:\n  - type: oauth2\n    config:\n")
	fmt.Fprintf(&b, "      clientId: client-%05d\n      clientSecret: sec:APP_%05d_CLIENT_SECRET\n", i, i)
	fmt.Fprintf(&b, "      redirectUris:\n        - var:APP_%05d_REDIRECT\n        - https://app-%d.test/cb\n", i, i)
	b.WriteString("      grantTypes:\n        - authorization_code\n        - refresh_token\n")
	b.WriteString("      responseTypes:\n        - code\n      tokenEndpointAuthMethod: client_secret_basic\n")
	b.WriteString("      pkceRequired: true\n      publicClient: false\n")
	b.WriteString("      scopes:\n        - openid\n        - profile")
	return b.String()
}

// benchFixture is a service whose gateway holds the first revision of a large bundle, and whose latest
// version is the second.
func benchFixture(b *testing.B) *versionFixture {
	b.Helper()
	keepVersions(b, 10)
	f := newVersionFixture(b)
	f.svc.(*versionService).projects = &fakeProjects{projects: map[string]string{"shop": "Shop"}}
	f.client.held = map[string]bool{}
	for i := 0; i < benchResources+25; i++ {
		f.client.held[fmt.Sprintf("APP_%05d_URL", i)] = true
		f.client.held[fmt.Sprintf("APP_%05d_CLIENT_SECRET", i)] = true
		f.client.held[fmt.Sprintf("APP_%05d_REDIRECT", i)] = true
	}
	f.exporter.response = benchBundle(0)
	_, svcErr := f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(b, svcErr)
	_, svcErr = f.svc.Apply(context.Background(), "gw-1", ApplyRequest{ConfirmRemovals: true})
	require.Nil(b, svcErr)
	// A few values the second revision refers to are missing, as a dry run then reports.
	for i := 0; i < benchResources+25; i += 50 {
		f.client.held[fmt.Sprintf("APP_%05d_REDIRECT", i)] = false
	}
	f.exporter.response = benchBundle(1)
	_, svcErr = f.svc.Capture(context.Background(), CaptureRequest{})
	require.Nil(b, svcErr)
	return f
}

// benchRequests are the reads a console makes of a gateway, each benchmarked with what the service
// keeps between requests and, cold, without it.
var benchRequests = map[string]func(svc VersionServiceInterface) *tidcommon.ServiceError{
	"Diff": func(svc VersionServiceInterface) *tidcommon.ServiceError {
		_, svcErr := svc.Diff(context.Background(), "gw-1", "", "")
		return svcErr
	},
	"DryRun": func(svc VersionServiceInterface) *tidcommon.ServiceError {
		_, svcErr := svc.Apply(context.Background(), "gw-1", ApplyRequest{DryRun: true})
		return svcErr
	},
	"ProjectStatus": func(svc VersionServiceInterface) *tidcommon.ServiceError {
		_, svcErr := svc.ProjectStatus(context.Background(), "gw-1", "shop", "", "")
		return svcErr
	},
	"Preview": func(svc VersionServiceInterface) *tidcommon.ServiceError {
		_, svcErr := svc.Preview(context.Background(), "gw-1", PreviewRequest{Project: "shop"})
		return svcErr
	},
}

// BenchmarkPlan measures each read of a large bundle against what the gateway holds.
func BenchmarkPlan(b *testing.B) {
	for _, name := range []string{"Diff", "DryRun", "ProjectStatus", "Preview"} {
		request := benchRequests[name]
		for _, cold := range []bool{false, true} {
			label := name
			if cold {
				label += "/cold"
			}
			b.Run(label, func(b *testing.B) {
				f := benchFixture(b)
				svc := f.svc.(*versionService)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if cold {
						svc.plans = newPlanCache()
					}
					if svcErr := request(f.svc); svcErr != nil {
						b.Fatal(svcErr)
					}
				}
			})
		}
	}
}
