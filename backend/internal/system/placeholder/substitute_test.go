// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package placeholder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubstitute_Ctx(t *testing.T) {
	resolve := MapResolve(map[string]string{"otpCode": "12<3"})

	// Plain text: no escaping.
	require.Equal(t, "Code: 12<3", Substitute("Code: {{ctx(otpCode)}}", CtxPattern, resolve, false))
	// HTML: substituted value escaped.
	require.Equal(t, "Code: 12&lt;3", Substitute("Code: {{ctx(otpCode)}}", CtxPattern, resolve, true))
	// Unknown key left literal.
	require.Equal(t, "Hi {{ctx(name)}}", Substitute("Hi {{ctx(name)}}", CtxPattern, resolve, false))
	// No placeholders: unchanged.
	require.Equal(t, "plain", Substitute("plain", CtxPattern, resolve, true))
	// Nil resolver leaves content untouched.
	require.Equal(t, "{{ctx(x)}}", Substitute("{{ctx(x)}}", CtxPattern, nil, false))
}

func TestPatterns_WhitespaceTolerance(t *testing.T) {
	ctx := MapResolve(map[string]string{"otp": "123"})
	// Whitespace around the argument is tolerated, matching the flow {{ctx(...)}} grammar.
	require.Equal(t, "123", Substitute("{{ctx(otp)}}", CtxPattern, ctx, false))
	require.Equal(t, "123", Substitute("{{ ctx( otp ) }}", CtxPattern, ctx, false))
}

func TestPatterns_DesignDotPathAndTKey(t *testing.T) {
	// design addresses a dotted token path; t addresses a dotted translation key.
	design := MapResolve(map[string]string{"palette.primary.main": "#1a73e8"})
	trans := MapResolve(map[string]string{"notification.otp.email.subject": "Your code"})

	require.Equal(t, "color:#1a73e8",
		Substitute("color:{{design(palette.primary.main)}}", DesignPattern, design, false))
	require.Equal(t, "Your code",
		Substitute("{{t(notification.otp.email.subject)}}", TPattern, trans, false))

	// A design pattern does not match a ctx/t placeholder and vice versa.
	require.Equal(t, "{{ctx(x)}}", Substitute("{{ctx(x)}}", DesignPattern, design, false))
}

func TestRender_AllThreeFamilies(t *testing.T) {
	ctx := MapResolve(map[string]string{"otp": "9&9"})
	trans := MapResolve(map[string]string{"greeting": "Hello"})
	design := MapResolve(map[string]string{"palette.primary.main": "#1a73e8"})

	body := `<p>{{t(greeting)}}</p><p>{{ctx(otp)}}</p>` +
		`<span style="color:{{design(palette.primary.main)}}">x</span>`

	// HTML body: ctx value escaped, all three families resolved.
	got := Render(body, true, ctx, trans, design)
	require.Equal(t,
		`<p>Hello</p><p>9&amp;9</p><span style="color:#1a73e8">x</span>`, got)

	// Subject style: no escaping, design skipped (nil) so its placeholder stays literal.
	subject := Render("{{t(greeting)}} {{design(palette.primary.main)}}", false, ctx, trans, nil)
	require.Equal(t, "Hello {{design(palette.primary.main)}}", subject)
}
