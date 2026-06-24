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

type OriginValidatorTestSuite struct {
	suite.Suite
}

func TestOriginValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(OriginValidatorTestSuite))
}

func (suite *OriginValidatorTestSuite) validate(raw string) error {
	return OriginValidator{}.Validate(json.RawMessage(raw))
}

func (suite *OriginValidatorTestSuite) TestLiteralsAndRegexOK() {
	err := suite.validate(`["https://app.example.com", {"regex":"^https://[a-z0-9-]+\\.example\\.com$"}]`)
	assert.NoError(suite.T(), err)
}

func (suite *OriginValidatorTestSuite) TestNullLiteralOK() {
	assert.NoError(suite.T(), suite.validate(`["null"]`))
}

func (suite *OriginValidatorTestSuite) TestEmptyArrayOK() {
	assert.NoError(suite.T(), suite.validate(`[]`))
}

func (suite *OriginValidatorTestSuite) TestMalformedJSON() {
	assert.Error(suite.T(), suite.validate(`{not an array`))
}

func (suite *OriginValidatorTestSuite) TestWildcardLiteralRejected() {
	assert.Error(suite.T(), suite.validate(`["*"]`))
}

func (suite *OriginValidatorTestSuite) TestBadRegexRejected() {
	assert.Error(suite.T(), suite.validate(`[{"regex":"("}]`))
}

func (suite *OriginValidatorTestSuite) TestRegexObjectMissingField() {
	assert.Error(suite.T(), suite.validate(`[{"foo":"bar"}]`))
}

func (suite *OriginValidatorTestSuite) TestOneBadEntryRejectsWhole() {
	assert.Error(suite.T(), suite.validate(`["https://app.example.com", "*"]`))
}
