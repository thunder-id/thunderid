// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package smoke

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/pack/packtest"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

type SmokeTestSuite struct {
	packtest.Suite
}

func TestSmokeTestSuite(t *testing.T) {
	suite.Run(t, new(SmokeTestSuite))
}

func (ts *SmokeTestSuite) TestEveryPlaneAnswers() {
	for name, baseURL := range map[string]string{
		"auth-dp":       testutils.PackAuthDPURL(),
		"data plane":    testutils.PackDPURL(),
		"control plane": testutils.PackCPURL(),
	} {
		status, raw := ts.Call(testutils.PackAnonymous(baseURL), http.MethodGet, "/health/liveness", nil, nil)
		ts.Equal(http.StatusOK, status, "%s: %s", name, raw)
	}
}

func (ts *SmokeTestSuite) TestEachPlaneAcceptsItsOwnToken() {
	status, raw := ts.Call(ts.CP, http.MethodGet, "/applications", nil, nil)
	ts.Equal(http.StatusOK, status, "the control plane refused the token auth-dp signed: %s", raw)

	status, raw = ts.Call(ts.DP, http.MethodGet, "/applications", nil, nil)
	ts.Equal(http.StatusOK, status, "the data plane refused the Admin Console token: %s", raw)

	// The planes do not share tokens: the control plane trusts auth-dp alone, and the data plane its own.
	status, _ = ts.Call(testutils.PackAnonymous(testutils.PackCPURL()), http.MethodGet, "/applications", nil, nil)
	ts.Equal(http.StatusUnauthorized, status, "the control plane answered a request without a token")
}
