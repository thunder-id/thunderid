// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// flow1CookieName is the wire name of the SSO cookie for "flow-1": "tid_sso_" followed by the first
// 16 hex characters of sha256("flow-1"). It is pinned so existing browser sessions keep working.
const flow1CookieName = "tid_sso_e865168f04ce2eb6"

type TransportTestSuite struct {
	suite.Suite
}

func TestTransportTestSuite(t *testing.T) {
	suite.Run(t, new(TransportTestSuite))
}

// cookieRequest returns a request carrying the given cookies.
func cookieRequest(cookies ...*http.Cookie) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/flow/execute", nil)
	for _, ck := range cookies {
		r.AddCookie(ck)
	}
	return r
}

func (s *TransportTestSuite) TestCookieName() {
	s.Equal(flow1CookieName, cookieName("flow-1"))
	s.Equal(cookieName("flow-1"), cookieName("flow-1"), "name must be stable for a flow")
	s.NotEqual(cookieName("flow-1"), cookieName("flow-2"), "different flows must get different cookie names")
}

func (s *TransportTestSuite) TestInbound_ContextRoundTrip() {
	ih := cookieInbound{flow1CookieName: "handle-1"}

	got, ok := InboundFrom(WithInbound(context.Background(), ih))
	s.True(ok)
	s.Equal("handle-1", got.HandleFor("flow-1"))
}

func (s *TransportTestSuite) TestInboundFrom_NothingAttached() {
	got, ok := InboundFrom(context.Background())
	s.False(ok)
	s.Nil(got)
}

func (s *TransportTestSuite) TestInboundFrom_NilAttached() {
	got, ok := InboundFrom(WithInbound(context.Background(), nil))
	s.False(ok)
	s.Nil(got)
}

func (s *TransportTestSuite) TestCookieTransport_Read_SelectsByFlowID() {
	transport := newCookieTransport(false)

	ih := transport.Read(&Exchange{Request: cookieRequest(
		&http.Cookie{Name: flow1CookieName, Value: "handle-1"},
		&http.Cookie{Name: cookieName("flow-2"), Value: "handle-2"},
		&http.Cookie{Name: "unrelated", Value: "x"},
	)})

	s.Equal("handle-1", ih.HandleFor("flow-1"))
	s.Equal("handle-2", ih.HandleFor("flow-2"))
	s.Equal("", ih.HandleFor("unrelated"), "a flow ID must not be used as a raw cookie name")
}

func (s *TransportTestSuite) TestCookieTransport_Read_Absent() {
	transport := newCookieTransport(false)

	s.Equal("", transport.Read(&Exchange{Request: cookieRequest(
		&http.Cookie{Name: "unrelated", Value: "x"})}).HandleFor("flow-1"))
	s.Equal("", transport.Read(&Exchange{Request: cookieRequest()}).HandleFor("flow-1"))
	s.Equal("", transport.Read(&Exchange{}).HandleFor("flow-1"), "a missing request carries no handle")
}

func (s *TransportTestSuite) TestCookieTransport_Write() {
	for _, secure := range []bool{true, false} {
		transport := newCookieTransport(secure)
		w := httptest.NewRecorder()

		transport.Write(&Exchange{Response: w}, "flow-1", "handle-1", time.Hour)

		cookies := w.Result().Cookies()
		s.Require().Len(cookies, 1)
		ck := cookies[0]
		s.Equal(flow1CookieName, ck.Name)
		s.Equal("handle-1", ck.Value)
		s.Equal("/", ck.Path)
		s.Equal(3600, ck.MaxAge)
		s.True(ck.HttpOnly)
		s.Equal(secure, ck.Secure)
		s.Equal(http.SameSiteLaxMode, ck.SameSite)
		s.Empty(ck.Domain)
	}
}

func (s *TransportTestSuite) TestCookieTransport_Clear() {
	for _, secure := range []bool{true, false} {
		transport := newCookieTransport(secure)
		w := httptest.NewRecorder()

		transport.Clear(&Exchange{Response: w}, "flow-1")

		cookies := w.Result().Cookies()
		s.Require().Len(cookies, 1)
		ck := cookies[0]
		s.Equal(flow1CookieName, ck.Name)
		s.Equal("", ck.Value)
		s.Equal("/", ck.Path)
		s.Equal(-1, ck.MaxAge)
		s.True(ck.HttpOnly)
		s.Equal(secure, ck.Secure)
		s.Equal(http.SameSiteLaxMode, ck.SameSite)
		s.Empty(ck.Domain)
		s.Contains(w.Header().Get("Set-Cookie"), "Max-Age=0")
	}
}

func (s *TransportTestSuite) TestCookieTransport_NoResponse() {
	transport := newCookieTransport(false)

	s.NotPanics(func() {
		transport.Write(&Exchange{}, "flow-1", "handle-1", time.Hour)
		transport.Clear(&Exchange{}, "flow-1")
	}, "a read-only exchange must be safe to write to")
}

// TestCookieTransport_RoundTrip writes a handle then reads it back through a second request,
// proving the transport's write and read agree on the cookie name.
func (s *TransportTestSuite) TestCookieTransport_RoundTrip() {
	transport := newCookieTransport(false)
	w := httptest.NewRecorder()
	transport.Write(&Exchange{Response: w}, "flow-1", "handle-xyz", time.Hour)

	ih := transport.Read(&Exchange{Request: cookieRequest(w.Result().Cookies()...)})
	s.Equal("handle-xyz", ih.HandleFor("flow-1"))
	s.Equal("", ih.HandleFor("flow-2"))
}

func (s *TransportTestSuite) TestNewHandleTransport_UsesConfig() {
	for _, secure := range []bool{true, false} {
		transport := NewHandleTransport(TransportConfig{SecureCookies: secure})
		w := httptest.NewRecorder()

		transport.Write(&Exchange{Response: w}, "flow-1", "handle-1", time.Hour)

		cookies := w.Result().Cookies()
		s.Require().Len(cookies, 1)
		s.Equal(flow1CookieName, cookies[0].Name)
		s.Equal(secure, cookies[0].Secure, "SecureCookies must reach the cookie")
		s.Equal("handle-1", transport.Read(&Exchange{Request: cookieRequest(cookies...)}).HandleFor("flow-1"))
	}
}
