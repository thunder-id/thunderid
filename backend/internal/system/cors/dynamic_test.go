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

	"github.com/thunder-id/thunderid/internal/system/log"
)

type DynamicMatcherTestSuite struct {
	suite.Suite
}

func TestDynamicMatcherTestSuite(t *testing.T) {
	suite.Run(t, new(DynamicMatcherTestSuite))
}

// TearDownTest clears the package-level dynamic matcher installed by the
// Initialize/Get tests so suites don't leak global state into one another.
func (suite *DynamicMatcherTestSuite) TearDownTest() {
	dynamicInstance = nil
}

// newDynamic builds a dynamicMatcher whose reader yields whatever raw/ok point
// to, so a test can change the stored config value between resolve() calls.
func (suite *DynamicMatcherTestSuite) newDynamic(raw *[]byte, ok *bool) *dynamicMatcher {
	return &dynamicMatcher{
		read:   func() ([]byte, bool) { return *raw, *ok },
		logger: log.GetLogger(),
	}
}

func (suite *DynamicMatcherTestSuite) allows(m *Matcher, origin string) bool {
	parsed, err := ParseOrigin(origin)
	suite.Require().NoError(err)
	allow, _ := m.Match(parsed)
	return allow
}

// --- JSON decode + compile round-trip ---

func (suite *DynamicMatcherTestSuite) TestJSONRoundTripCompiles() {
	var entries OriginEntries
	err := json.Unmarshal(
		[]byte(`["https://app.example.com", {"regex":"^https://[a-z0-9-]+\\.example\\.com$"}]`), &entries)
	suite.Require().NoError(err)
	suite.Require().Len(entries, 2)

	m, err := CompileMatcher(entries)
	suite.Require().NoError(err)
	assert.True(suite.T(), suite.allows(m, "https://app.example.com"))
	assert.True(suite.T(), suite.allows(m, "https://foo-bar.example.com"))
	assert.False(suite.T(), suite.allows(m, "https://evil.com"))
}

func (suite *DynamicMatcherTestSuite) TestUnmarshalJSONNonArray() {
	var entries OriginEntries
	assert.Error(suite.T(), json.Unmarshal([]byte(`{"a":1}`), &entries))
}

func (suite *DynamicMatcherTestSuite) TestUnmarshalJSONNull() {
	var entries OriginEntries
	assert.Error(suite.T(), json.Unmarshal([]byte(`null`), &entries))
}

func (suite *DynamicMatcherTestSuite) TestUnmarshalJSONRegexMissingField() {
	var entries OriginEntries
	assert.Error(suite.T(), json.Unmarshal([]byte(`[{"foo":"bar"}]`), &entries))
}

func (suite *DynamicMatcherTestSuite) TestUnmarshalJSONWrongElementType() {
	var entries OriginEntries
	assert.Error(suite.T(), json.Unmarshal([]byte(`[123]`), &entries))
}

// --- Memoized resolve() ---

func (suite *DynamicMatcherTestSuite) TestResolveFirstCallCompiles() {
	raw, ok := []byte(`["https://example.com"]`), true
	m := suite.newDynamic(&raw, &ok).resolve()
	suite.Require().NotNil(m)
	assert.True(suite.T(), suite.allows(m, "https://example.com"))
}

func (suite *DynamicMatcherTestSuite) TestResolveUnchangedReusesMatcher() {
	raw, ok := []byte(`["https://example.com"]`), true
	d := suite.newDynamic(&raw, &ok)
	m1 := d.resolve()
	m2 := d.resolve()
	assert.Same(suite.T(), m1, m2)
}

func (suite *DynamicMatcherTestSuite) TestResolveChangedRecompiles() {
	raw, ok := []byte(`["https://example.com"]`), true
	d := suite.newDynamic(&raw, &ok)
	m1 := d.resolve()

	raw = []byte(`["https://test.com"]`)
	m2 := d.resolve()

	assert.NotSame(suite.T(), m1, m2)
	assert.True(suite.T(), suite.allows(m2, "https://test.com"))
	assert.False(suite.T(), suite.allows(m2, "https://example.com"))
}

func (suite *DynamicMatcherTestSuite) TestResolveNotSetReturnsNil() {
	raw, ok := []byte(nil), false
	assert.Nil(suite.T(), suite.newDynamic(&raw, &ok).resolve())
}

func (suite *DynamicMatcherTestSuite) TestResolveNotSetAfterGoodKeepsLast() {
	raw, ok := []byte(`["https://example.com"]`), true
	d := suite.newDynamic(&raw, &ok)
	m1 := d.resolve()

	ok = false
	m2 := d.resolve()
	assert.Same(suite.T(), m1, m2)
}

func (suite *DynamicMatcherTestSuite) TestResolveBadJSONKeepsLast() {
	raw, ok := []byte(`["https://example.com"]`), true
	d := suite.newDynamic(&raw, &ok)
	m1 := d.resolve()

	raw = []byte(`{not json`)
	m2 := d.resolve()
	assert.Same(suite.T(), m1, m2)
}

func (suite *DynamicMatcherTestSuite) TestResolveCompileErrorKeepsLast() {
	raw, ok := []byte(`["https://example.com"]`), true
	d := suite.newDynamic(&raw, &ok)
	m1 := d.resolve()

	// "*" decodes as a literal but fails to compile (wildcard literal).
	raw = []byte(`["*"]`)
	m2 := d.resolve()
	assert.Same(suite.T(), m1, m2)
}

// --- Package-level install / read ---

func (suite *DynamicMatcherTestSuite) TestGetDynamicMatcherNilBeforeInit() {
	dynamicInstance = nil
	assert.Nil(suite.T(), GetDynamicMatcher())
}

func (suite *DynamicMatcherTestSuite) TestInitializeAndGet() {
	raw, ok := []byte(`["https://example.com"]`), true
	InitializeDynamicMatcher(func() ([]byte, bool) { return raw, ok })

	m := GetDynamicMatcher()
	suite.Require().NotNil(m)
	assert.True(suite.T(), suite.allows(m, "https://example.com"))
}
