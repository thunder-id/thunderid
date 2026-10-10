// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type StoreTestSuite struct {
	suite.Suite
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) TestBuildTemplateFromRow_EmailWithDesign() {
	// Happy path: a full email row (string CONTENT, as returned by SQLite) with a design JSON document.
	row := map[string]interface{}{
		"id":           "1",
		"channel":      string(ChannelTypeEmail),
		"handle":       "otp-verification",
		"display_name": "OTP Verification",
		"description":  "desc",
		"content":      `{"subject":"s.key","body":"<p>{{ctx(otp)}}</p>"}`,
		"design":       `{"colorScheme":"dark"}`,
	}

	dao, err := buildTemplateFromRow(row)
	s.Require().NoError(err)
	s.Require().Equal("1", dao.ID)
	s.Require().Equal(ChannelTypeEmail, dao.Channel)
	s.Require().Equal("otp-verification", dao.Handle)
	s.Require().Equal("OTP Verification", dao.DisplayName)
	s.Require().Equal("desc", dao.Description)
	s.Require().Equal("s.key", dao.Content.Subject)
	s.Require().Equal("<p>{{ctx(otp)}}</p>", dao.Content.Body)
	s.Require().NotNil(dao.Design)
	s.Require().Equal(colorSchemeDark, dao.Design.ColorScheme)
}

func (s *StoreTestSuite) TestBuildTemplateFromRow_NoDesign_ByteContent() {
	// Happy path: an SMS row with []byte CONTENT (as returned by PostgreSQL) and an empty ({}) design.
	row := map[string]interface{}{
		"id":           "2",
		"channel":      string(ChannelTypeSMS),
		"handle":       "otp",
		"display_name": "OTP",
		"description":  nil,
		"content":      []byte(`{"body":"{{ctx(otp)}}"}`),
		"design":       `{}`,
	}

	dao, err := buildTemplateFromRow(row)
	s.Require().NoError(err)
	s.Require().Equal("{{ctx(otp)}}", dao.Content.Body)
	s.Require().Empty(dao.Content.Subject)
	s.Require().Empty(dao.Description)
	s.Require().Nil(dao.Design)
}

func (s *StoreTestSuite) TestBuildTemplateFromRow_Errors() {
	base := func() map[string]interface{} {
		return map[string]interface{}{
			"id":           "1",
			"channel":      string(ChannelTypeEmail),
			"handle":       "h",
			"display_name": "D",
			"content":      `{"body":"b"}`,
		}
	}

	cases := map[string]func(map[string]interface{}){
		"missing id":             func(r map[string]interface{}) { delete(r, "id") },
		"id wrong type":          func(r map[string]interface{}) { r["id"] = 123 },
		"missing channel":        func(r map[string]interface{}) { delete(r, "channel") },
		"missing handle":         func(r map[string]interface{}) { delete(r, "handle") },
		"missing display_name":   func(r map[string]interface{}) { delete(r, "display_name") },
		"malformed content json": func(r map[string]interface{}) { r["content"] = `{not-json` },
	}

	for name, mutate := range cases {
		s.Run(name, func() {
			row := base()
			mutate(row)
			_, err := buildTemplateFromRow(row)
			s.Require().Error(err)
		})
	}
}

func (s *StoreTestSuite) TestParseCountResult() {
	// Happy: the "total" alias across the driver numeric types.
	n, err := parseCountResult([]map[string]interface{}{{"total": int64(5)}})
	s.Require().NoError(err)
	s.Require().Equal(5, n)

	n, err = parseCountResult([]map[string]interface{}{{"total": int(3)}})
	s.Require().NoError(err)
	s.Require().Equal(3, n)

	n, err = parseCountResult([]map[string]interface{}{{"total": float64(9)}})
	s.Require().NoError(err)
	s.Require().Equal(9, n)

	// Unhappy: no rows, missing "total" field, unexpected type.
	_, err = parseCountResult(nil)
	s.Require().Error(err)

	_, err = parseCountResult([]map[string]interface{}{{"count": 1}})
	s.Require().Error(err)

	_, err = parseCountResult([]map[string]interface{}{{"total": "5"}})
	s.Require().Error(err)
}

func (s *StoreTestSuite) TestJSONColumnToString() {
	s.Require().Equal("abc", jsonColumnToString("abc"))
	s.Require().Equal("xy", jsonColumnToString([]byte("xy")))
	s.Require().Equal("", jsonColumnToString(123))
	s.Require().Equal("", jsonColumnToString(nil))
}

func (s *StoreTestSuite) TestStringOrEmpty() {
	s.Require().Equal("s", stringOrEmpty("s"))
	s.Require().Equal("", stringOrEmpty(nil))
	s.Require().Equal("", stringOrEmpty(42))
}
