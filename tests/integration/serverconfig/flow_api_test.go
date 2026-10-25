// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const flowSection = "flow"

// flowTypeValue mirrors one flow type's entry in the flow section value.
type flowTypeValue struct {
	DefaultHandle string `json:"defaultHandle"`
	ExpirySeconds int64  `json:"expirySeconds"`
}

// FlowConfigAPITestSuite covers the selfFlow entry of the flow server-config section: it takes an
// expiry, and refuses a negative expiry and a default handle.
type FlowConfigAPITestSuite struct {
	suite.Suite
	// original is the writable flow layer before the suite, restored before each test and at the end.
	original json.RawMessage
}

func TestFlowConfigAPITestSuite(t *testing.T) {
	suite.Run(t, new(FlowConfigAPITestSuite))
}

func (suite *FlowConfigAPITestSuite) SetupSuite() {
	original, err := testutils.GetWritableServerConfig(flowSection)
	suite.Require().NoError(err)
	suite.original = original
	t := suite.T()
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowSection, original); err != nil {
			t.Errorf("cleanup: failed to restore the flow section: %v", err)
		}
	})
}

// SetupTest starts each test from the layer the suite found.
func (suite *FlowConfigAPITestSuite) SetupTest() {
	suite.Require().NoError(testutils.PutWritableServerConfig(flowSection, suite.original))
}

func (suite *FlowConfigAPITestSuite) TestPutSelfFlowExpiryPersistsAndReadsBack() {
	status, body := suite.putSelfFlow(flowTypeValue{ExpirySeconds: 600})
	suite.Require().Equal(http.StatusOK, status, "body: %s", body)

	suite.Equal(int64(600), suite.writableSelfFlow().ExpirySeconds)
}

// TestPutInvalidSelfFlowReturns400 writes a valid expiry first, so a refused write that stored its
// value would show in the layer.
func (suite *FlowConfigAPITestSuite) TestPutInvalidSelfFlowReturns400() {
	for name, value := range map[string]flowTypeValue{
		"negative expiry": {ExpirySeconds: -1},
		"default handle":  {DefaultHandle: "default-self-attribute-verification-flow", ExpirySeconds: 900},
	} {
		suite.Run(name, func() {
			status, body := suite.putSelfFlow(flowTypeValue{ExpirySeconds: 600})
			suite.Require().Equal(http.StatusOK, status, "precondition: a valid expiry is stored: %s", body)

			status, body = suite.putSelfFlow(value)

			suite.Equal(http.StatusBadRequest, status, "body: %s", body)
			suite.Equal("SCF-1003", suite.errorCode(body))
			suite.Equal(flowTypeValue{ExpirySeconds: 600}, suite.writableSelfFlow(),
				"a rejected value must not persist")
		})
	}
}

// --- helpers ---

// putSelfFlow writes value as selfFlow over the suite's original layer, keeping its other keys.
func (suite *FlowConfigAPITestSuite) putSelfFlow(value flowTypeValue) (int, []byte) {
	section := map[string]interface{}{}
	suite.Require().NoError(json.Unmarshal(suite.original, &section))
	section["selfFlow"] = value
	payload, err := json.Marshal(section)
	suite.Require().NoError(err)

	status, body, err := testutils.PutWritableServerConfigWithStatus(flowSection, payload)
	suite.Require().NoError(err)
	return status, body
}

// writableSelfFlow reads selfFlow from the writable flow layer.
func (suite *FlowConfigAPITestSuite) writableSelfFlow() flowTypeValue {
	layer, err := testutils.GetWritableServerConfig(flowSection)
	suite.Require().NoError(err)

	var section struct {
		SelfFlow flowTypeValue `json:"selfFlow"`
	}
	suite.Require().NoError(json.Unmarshal(layer, &section), "layer: %s", layer)
	return section.SelfFlow
}

func (suite *FlowConfigAPITestSuite) errorCode(body []byte) string {
	var errResp apiErrorResponse
	suite.Require().NoError(json.Unmarshal(body, &errResp), "body: %s", body)
	return errResp.Code
}
