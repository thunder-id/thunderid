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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"
)

type CompilerTestSuite struct {
	suite.Suite
}

func TestCompilerTestSuite(t *testing.T) {
	suite.Run(t, new(CompilerTestSuite))
}

func (suite *CompilerTestSuite) TestCompileLiteralEntry() {
	rule, err := compile(literalEntry{Value: "https://example.com"})
	suite.Require().NoError(err)
	assert.Equal(suite.T(), kindLiteral, rule.kind())
}

func (suite *CompilerTestSuite) TestCompileRegexEntry() {
	rule, err := compile(regexEntry{Pattern: `^https://.+$`})
	suite.Require().NoError(err)
	assert.Equal(suite.T(), kindRegex, rule.kind())
}

func (suite *CompilerTestSuite) TestCompileLiteralWhitespaceTrimmed() {
	rule, err := compile(literalEntry{Value: "  https://example.com  "})
	suite.Require().NoError(err)
	m := newMatcher([]originRule{rule})
	parsed, err := ParseOrigin("https://example.com")
	suite.Require().NoError(err)
	allow, _ := m.Match(parsed)
	assert.True(suite.T(), allow)
}

func (suite *CompilerTestSuite) TestCompileNullLiteral() {
	rule, err := compile(literalEntry{Value: "null"})
	suite.Require().NoError(err)
	m := newMatcher([]originRule{rule})
	allowNull, _ := m.Match(ParseResult{Raw: "null", IsNull: true})
	assert.True(suite.T(), allowNull)
	parsed, err := ParseOrigin("https://example.com")
	suite.Require().NoError(err)
	allowOrigin, _ := m.Match(parsed)
	assert.False(suite.T(), allowOrigin)
}

func (suite *CompilerTestSuite) TestCompileWildcardLiteralRejected() {
	_, err := compile(literalEntry{Value: "*"})
	suite.Require().Error(err)
	assert.True(suite.T(), errors.Is(err, ErrWildcardLiteral))
}

func (suite *CompilerTestSuite) TestisRegexAnchored() {
	cases := []struct {
		pattern  string
		anchored bool
	}{
		{`^https://example\.com$`, true},
		{`\Ahttps://example\.com\z`, true},
		{`^https://example\.com\z`, true},
		{`\Ahttps://example\.com$`, true},
		{`https://example\.com$`, false},
		{`^https://example\.com`, false},
		{`https://example\.com`, false},
		{`.*\.example\.com`, false},
	}
	for _, c := range cases {
		assert.Equal(suite.T(), c.anchored, isRegexAnchored(c.pattern), c.pattern)
	}
}

func (suite *CompilerTestSuite) TestCompileEmptyLiteralRejected() {
	_, err := compile(literalEntry{Value: "   "})
	suite.Require().Error(err)
	assert.True(suite.T(), errors.Is(err, ErrEmptyEntry))
}

func (suite *CompilerTestSuite) TestCompileInvalidLiteralRejected() {
	_, err := compile(literalEntry{Value: "not-a-url"})
	suite.Require().Error(err)
	assert.True(suite.T(), errors.Is(err, ErrInvalidLiteral))
}

func (suite *CompilerTestSuite) TestCompileEmptyRegexRejected() {
	_, err := compile(regexEntry{Pattern: ""})
	suite.Require().Error(err)
	assert.True(suite.T(), errors.Is(err, ErrEmptyEntry))
}

func (suite *CompilerTestSuite) TestCompileInvalidRegexRejected() {
	_, err := compile(regexEntry{Pattern: "([unterminated"})
	suite.Require().Error(err)
	assert.True(suite.T(), errors.Is(err, ErrInvalidRegex))
}

type unknownEntry struct{}

func (unknownEntry) isOriginEntry() {}

func (suite *CompilerTestSuite) TestCompileUnknownEntryTypeRejected() {
	_, err := compile(unknownEntry{})
	suite.Require().Error(err)
}

func (suite *CompilerTestSuite) TestCompileAllEmptyInput() {
	rules, err := compileAll(nil)
	suite.Require().NoError(err)
	assert.Nil(suite.T(), rules)
}

func (suite *CompilerTestSuite) TestCompileAllPreservesOrder() {
	rules, err := compileAll([]entry{
		literalEntry{Value: "https://a.com"},
		regexEntry{Pattern: `^https://b\.com$`},
		literalEntry{Value: "https://c.com"},
	})
	suite.Require().NoError(err)
	suite.Require().Len(rules, 3)
	assert.Equal(suite.T(), kindLiteral, rules[0].kind())
	assert.Equal(suite.T(), kindRegex, rules[1].kind())
	assert.Equal(suite.T(), kindLiteral, rules[2].kind())
}

func (suite *CompilerTestSuite) TestCompileAllFailsFastWithIndex() {
	_, err := compileAll([]entry{
		literalEntry{Value: "https://ok.com"},
		regexEntry{Pattern: "([bad"},
	})
	suite.Require().Error(err)
	assert.Contains(suite.T(), err.Error(), "allowedOrigins[1]")
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLLiteralEntries() {
	doc := []byte(`
- https://example.com
- https://other.com
`)
	var entries OriginEntries
	suite.Require().NoError(yaml.Unmarshal(doc, &entries))
	suite.Require().Len(entries, 2)
	assert.IsType(suite.T(), literalEntry{}, entries[0])
	assert.IsType(suite.T(), literalEntry{}, entries[1])
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLRegexEntries() {
	doc := []byte(`
- regex: '^https://[a-z]+\.example\.com$'
`)
	var entries OriginEntries
	suite.Require().NoError(yaml.Unmarshal(doc, &entries))
	suite.Require().Len(entries, 1)
	r, ok := entries[0].(regexEntry)
	suite.Require().True(ok)
	assert.Equal(suite.T(), `^https://[a-z]+\.example\.com$`, r.Pattern)
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLMixedEntries() {
	doc := []byte(`
- https://example.com
- regex: '^https://.+\.staging\.example\.com$'
- "null"
`)
	var entries OriginEntries
	suite.Require().NoError(yaml.Unmarshal(doc, &entries))
	suite.Require().Len(entries, 3)
	assert.IsType(suite.T(), literalEntry{}, entries[0])
	assert.IsType(suite.T(), regexEntry{}, entries[1])
	assert.IsType(suite.T(), literalEntry{}, entries[2])
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLNonSequenceRejected() {
	doc := []byte(`foo: bar`)
	var entries OriginEntries
	err := yaml.Unmarshal(doc, &entries)
	suite.Require().Error(err)
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLRegexMissingFieldRejected() {
	doc := []byte(`
- pattern: foo
`)
	var entries OriginEntries
	err := yaml.Unmarshal(doc, &entries)
	suite.Require().Error(err)
}

func (suite *CompilerTestSuite) TestUnmarshalYAMLUnsupportedNodeRejected() {
	doc := []byte(`
- - nested
`)
	var entries OriginEntries
	err := yaml.Unmarshal(doc, &entries)
	suite.Require().Error(err)
}
