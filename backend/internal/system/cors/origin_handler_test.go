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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cors

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type OriginHandlerTestSuite struct {
	suite.Suite
}

func TestOriginHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(OriginHandlerTestSuite))
}

// wrap nests an allowed-origins array literal in the object-shaped section value the handler expects.
func wrap(allowedOrigins string) json.RawMessage {
	return json.RawMessage(`{"allowedOrigins":` + allowedOrigins + `}`)
}

func (suite *OriginHandlerTestSuite) validate(raw string) error {
	v := OriginHandler{}
	decoded, err := v.Decode(wrap(raw))
	if err != nil {
		return err
	}
	return v.Validate(decoded, nil, nil)
}

func (suite *OriginHandlerTestSuite) TestLiteralsAndRegexOK() {
	err := suite.validate(`["https://app.example.com", {"regex":"^https://[a-z0-9-]+\\.example\\.com$"}]`)
	assert.NoError(suite.T(), err)
}

func (suite *OriginHandlerTestSuite) TestNullLiteralOK() {
	assert.NoError(suite.T(), suite.validate(`["null"]`))
}

func (suite *OriginHandlerTestSuite) TestEmptyArrayOK() {
	assert.NoError(suite.T(), suite.validate(`[]`))
}

func (suite *OriginHandlerTestSuite) TestMalformedJSON() {
	assert.Error(suite.T(), suite.validate(`{not an array`))
}

func (suite *OriginHandlerTestSuite) TestWildcardLiteralRejected() {
	assert.Error(suite.T(), suite.validate(`["*"]`))
}

func (suite *OriginHandlerTestSuite) TestBadRegexRejected() {
	assert.Error(suite.T(), suite.validate(`[{"regex":"("}]`))
}

func (suite *OriginHandlerTestSuite) TestRegexObjectMissingField() {
	assert.Error(suite.T(), suite.validate(`[{"foo":"bar"}]`))
}

func (suite *OriginHandlerTestSuite) TestOneBadEntryRejectsWhole() {
	assert.Error(suite.T(), suite.validate(`["https://app.example.com", "*"]`))
}

// --- Merge ---

func (suite *OriginHandlerTestSuite) merge(readOnly, writable string) string {
	merged := OriginHandler{}.Merge(suite.decode(readOnly), suite.decode(writable))
	out, err := json.Marshal(merged)
	suite.Require().NoError(err)
	return string(out)
}

func (suite *OriginHandlerTestSuite) decode(s string) any {
	if s == "" {
		return nil
	}
	v, err := OriginHandler{}.Decode(wrap(s))
	suite.Require().NoError(err)
	return v
}

func (suite *OriginHandlerTestSuite) TestMergeUnionDeduplicates() {
	assert.JSONEq(suite.T(), `{"allowedOrigins":["a","b","c"]}`, suite.merge(`["a","b"]`, `["b","c"]`))
}

func (suite *OriginHandlerTestSuite) TestMergeReadOnlyFirst() {
	assert.Equal(suite.T(), `{"allowedOrigins":["https://static.example.com","https://app.example.com"]}`,
		suite.merge(`["https://static.example.com"]`, `["https://app.example.com"]`))
}

func (suite *OriginHandlerTestSuite) TestMergeEmptyLayers() {
	assert.Equal(suite.T(), `{"allowedOrigins":[]}`, suite.merge("", ""))
}

func (suite *OriginHandlerTestSuite) TestMergeOnlyWritable() {
	assert.Equal(suite.T(), `{"allowedOrigins":["a"]}`, suite.merge("", `["a"]`))
}

func (suite *OriginHandlerTestSuite) TestMergeRegexEntryMarshals() {
	assert.JSONEq(suite.T(), `{"allowedOrigins":[{"regex":"^https://x$"}]}`,
		suite.merge(`[{"regex":"^https://x$"}]`, ""))
}

// --- Decode ---

func (suite *OriginHandlerTestSuite) TestDecodeEmptyYieldsEmptyAllowedOrigins() {
	decoded, err := OriginHandler{}.Decode(json.RawMessage(nil))
	suite.Require().NoError(err)

	out, err := json.Marshal(decoded)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), `{"allowedOrigins":[]}`, string(out))
}

func (suite *OriginHandlerTestSuite) TestDecodeMissingKeyYieldsEmptyAllowedOrigins() {
	decoded, err := OriginHandler{}.Decode(json.RawMessage(`{}`))
	suite.Require().NoError(err)

	out, err := json.Marshal(decoded)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), `{"allowedOrigins":[]}`, string(out))
}

func (suite *OriginHandlerTestSuite) TestDecodeExplicitNullRejected() {
	_, err := OriginHandler{}.Decode(json.RawMessage(`{"allowedOrigins":null}`))
	assert.Error(suite.T(), err)
}

func (suite *OriginHandlerTestSuite) TestDecodeBareListRejected() {
	_, err := OriginHandler{}.Decode(json.RawMessage(`["https://app.example.com"]`))
	assert.Error(suite.T(), err)
}
