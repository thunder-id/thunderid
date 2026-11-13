// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controlplane

import "net/http"

// The SSO session timeouts are authored on a Control Plane, though it runs no sessions: the section is
// served with its defaults in effect, and an export carries only what was set, so a gateway's own
// defaults apply to the rest.
func (ts *ControlPlaneTestSuite) TestTheSessionSectionIsAuthoredHere() {
	var names []string
	status, raw := ts.call(http.MethodGet, "/server-config", nil, &names)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Contains(names, "session", "the Control Plane must serve the session section")

	var layers struct {
		Merged map[string]int64 `json:"merged"`
	}
	status, raw = ts.call(http.MethodGet, "/server-config/session", nil, &layers)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Equal(map[string]int64{"idleTimeoutSeconds": 7200, "absoluteTimeoutSeconds": 28800}, layers.Merged)

	defer ts.call(http.MethodPut, "/server-config/session", map[string]any{}, nil)
	status, raw = ts.call(http.MethodPut, "/server-config/session", map[string]any{"idleTimeoutSeconds": 1800}, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))

	var exported struct {
		Resources string `json:"resources"`
	}
	status, raw = ts.call(http.MethodPost, "/export", map[string]any{"serverConfigs": []string{"session"}},
		&exported)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	ts.Regexp(`idleTimeoutSeconds"?:\s*1800\b`, exported.Resources, "the export must carry the written timeout")
	ts.NotContains(exported.Resources, "absoluteTimeoutSeconds", "the export must carry only what was set")
}
