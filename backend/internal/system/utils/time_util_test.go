// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type TimeUtilTestSuite struct {
	suite.Suite
}

func TestTimeUtilTestSuite(t *testing.T) {
	suite.Run(t, new(TimeUtilTestSuite))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_TimeValue() {
	input := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), input, result)
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_TimeValueNonUTCNormalized() {
	loc := time.FixedZone("IST", 5*3600+30*60)
	input := time.Date(2026, 6, 2, 15, 30, 0, 0, loc)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), time.UTC, result.Location())
	assert.True(suite.T(), input.Equal(result))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_SQLiteFormat() {
	input := "2026-06-02 21:57:49.157215"
	expected := time.Date(2026, 6, 2, 21, 57, 49, 157215000, time.UTC)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), expected.Equal(result))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_SQLiteFormatWithTrailingTokens() {
	// Go's time.Time.String() appends zone and monotonic tokens; only date+time portion is used.
	input := "2026-06-02 21:57:49.157215 +0000 UTC m=+123.456"
	expected := time.Date(2026, 6, 2, 21, 57, 49, 157215000, time.UTC)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), expected.Equal(result))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_ISO8601UTC() {
	input := "2026-06-02T21:57:49Z"
	expected := time.Date(2026, 6, 2, 21, 57, 49, 0, time.UTC)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), expected.Equal(result))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_ISO8601WithOffset() {
	input := "2023-12-01T10:30:45+05:30"
	parsed, _ := time.Parse("2006-01-02T15:04:05Z07:00", input)
	result, err := ParseDBTimeField(input, "field")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), parsed.Equal(result))
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_UnexpectedType() {
	_, err := ParseDBTimeField(12345, "field")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected type for field")
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_UnparsableString() {
	_, err := ParseDBTimeField("not-a-date", "field")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error parsing field")
}

func (suite *TimeUtilTestSuite) TestParseDBTimeField_ResultIsUTC() {
	result, err := ParseDBTimeField("2026-06-02 10:00:00", "field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), time.UTC, result.Location())
}

func (suite *TimeUtilTestSuite) TestFormatRFC3339() {
	suite.Empty(FormatRFC3339(time.Time{}), "the zero time renders as empty")
	suite.Equal("2026-06-16T10:00:00Z",
		FormatRFC3339(time.Date(2026, 6, 16, 15, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))),
		"the time is rendered in UTC")
}
