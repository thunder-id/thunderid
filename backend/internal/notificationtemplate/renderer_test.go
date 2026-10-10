// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	translationmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// designTestRegex matches the design placeholders the design mock substitutes.
var designTestRegex = regexp.MustCompile(`\{\{design\(([A-Za-z0-9_.-]+)\)\}\}`)

// RendererTestSuite tests the runtime templateRenderer with a mocked store and mocked resolvers.
type RendererTestSuite struct {
	suite.Suite
	mockStore *notificationTemplateStoreInterfaceMock
	ctx       context.Context
}

func TestRendererTestSuite(t *testing.T) {
	suite.Run(t, new(RendererTestSuite))
}

func (s *RendererTestSuite) SetupTest() {
	s.mockStore = newNotificationTemplateStoreInterfaceMock(s.T())
	s.ctx = context.Background()
}

// expectGet stubs the store to return dao for a single GetTemplateByHandle on (channel, handle).
func (s *RendererTestSuite) expectGet(channel ChannelType, handle string, dao templateDAO) {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, channel, handle).Return(dao, nil).Once()
}

// translator returns a mock translationResolver that resolves keys from vals; an unknown key reports
// not-found. The expectation is lenient (Maybe), so a field with no {{t}} placeholder is fine.
func (s *RendererTestSuite) translator(vals map[string]string) *translationResolverMock {
	m := newTranslationResolverMock(s.T())
	m.EXPECT().ResolveTranslationsForKey(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, lang, ns, key string) (
			*translationmgt.TranslationResponse, *tidcommon.ServiceError) {
			if v, ok := vals[key]; ok {
				return &translationmgt.TranslationResponse{Language: lang, Namespace: ns, Key: key, Value: v}, nil
			}
			return nil, &translationmgt.ErrorTranslationNotFound
		}).Maybe()
	return m
}

// failingTranslator returns a mock translationResolver that always fails with err.
func (s *RendererTestSuite) failingTranslator(err *tidcommon.ServiceError) *translationResolverMock {
	m := newTranslationResolverMock(s.T())
	m.EXPECT().ResolveTranslationsForKey(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, err).Maybe()
	return m
}

// designer returns a mock designResolver that substitutes known tokens and leaves unknown ones in
// place, mirroring the real design resolve service.
func (s *RendererTestSuite) designer(vals map[string]string) *designResolverMock {
	m := newDesignResolverMock(s.T())
	m.EXPECT().ResolveDesignContent(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _, _, content string) (string, *tidcommon.ServiceError) {
			return designTestRegex.ReplaceAllStringFunc(content, func(match string) string {
				token := designTestRegex.FindStringSubmatch(match)[1]
				if v, ok := vals[token]; ok {
					return v
				}
				return match
			}), nil
		}).Maybe()
	return m
}

// failingDesigner returns a mock designResolver that always returns a service error.
func (s *RendererTestSuite) failingDesigner() *designResolverMock {
	m := newDesignResolverMock(s.T())
	m.EXPECT().ResolveDesignContent(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("", &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "DSN-ERR"}).Maybe()
	return m
}

// newRenderer builds a renderer over the mocked store, wiring the translation resolver and the design
// resolver (when given) the same way the service manager does.
func (s *RendererTestSuite) newRenderer(tr translationResolver, dr designResolver) *templateRenderer {
	p := newTemplateRenderer(s.mockStore, tr)
	if dr != nil {
		p.SetDesignResolver(dr)
	}
	return p
}

func (s *RendererTestSuite) TestResolve_Email() {
	s.expectGet(ChannelTypeEmail, "otp-verification", templateDAO{
		ID: "t1", Channel: ChannelTypeEmail, Handle: "otp-verification", DisplayName: "OTP",
		Content: TemplateContent{
			Subject: "{{t(otp.subject)}}",
			Body:    "<b>{{t(otp.body)}}</b> {{ctx(otpCode)}} {{design(palette.primary.main)}}",
		},
		Design: &TemplateDesign{ColorScheme: colorSchemeDark},
	})
	tr := s.translator(map[string]string{
		"otp.subject": "Your verification code",
		"otp.body":    "Code",
	})
	dr := s.designer(map[string]string{"palette.primary.main": "#111"})
	p := s.newRenderer(tr, dr)

	rc, err := p.Resolve(s.ctx, "email", "otp-verification", RenderInput{
		Locale:  "en-US",
		Data:    map[string]string{"otpCode": "123"},
		ThemeID: "theme1",
	})
	s.Require().Nil(err)
	s.Equal("Your verification code", rc.Subject)
	s.Equal("<b>Code</b> 123 #111", rc.Body)
	// The stored template's color scheme is passed through to the design resolver.
	dr.AssertCalled(s.T(), "ResolveDesignContent", mock.Anything, mock.Anything, colorSchemeDark, mock.Anything)
}

// TestResolve_ColorSchemeEmptyWhenUnset verifies an email template with no explicit scheme passes an
// empty scheme, so the design service falls back to the theme's own defaultColorScheme.
func (s *RendererTestSuite) TestResolve_ColorSchemeEmptyWhenUnset() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "t1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "s", Body: "{{design(palette.primary.main)}}"},
	})
	dr := s.designer(map[string]string{"palette.primary.main": "#fff"})
	p := s.newRenderer(s.translator(nil), dr)

	rc, err := p.Resolve(s.ctx, "email", "h", RenderInput{ThemeID: "theme1"})
	s.Require().Nil(err)
	s.Equal("#fff", rc.Body)
	dr.AssertCalled(s.T(), "ResolveDesignContent", mock.Anything, mock.Anything, "", mock.Anything)
}

// TestResolve_MultipleAndNoKeys covers a field with several {{t}} keys and static tail text.
func (s *RendererTestSuite) TestResolve_MultipleAndNoKeys() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "t1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(greeting)}}, {{t(closing)}} — static tail"},
	})
	p := s.newRenderer(s.translator(map[string]string{"greeting": "Hello", "closing": "bye"}), nil)

	rc, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().Nil(err)
	s.Equal("Hello, bye — static tail", rc.Body)
}

// TestResolve_SMS covers a translated value that itself embeds a {{ctx}} placeholder, resolved in the
// later context pass, and confirms SMS carries no subject.
func (s *RendererTestSuite) TestResolve_SMS() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(sms.body)}}"},
	})
	p := s.newRenderer(s.translator(map[string]string{"sms.body": "Code {{ctx(otpCode)}}"}), nil)

	rc, err := p.Resolve(s.ctx, "sms", "h", RenderInput{Data: map[string]string{"otpCode": "123"}})
	s.Require().Nil(err)
	s.Empty(rc.Subject)
	s.Equal("Code 123", rc.Body)
}

// TestResolve_DottedContextKey covers a {{ctx}} key containing a dot, matching the placeholder grammar.
func (s *RendererTestSuite) TestResolve_DottedContextKey() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "Hi {{ctx(user.name)}}"},
	})
	p := s.newRenderer(s.translator(nil), nil)

	rc, err := p.Resolve(s.ctx, "sms", "h", RenderInput{Data: map[string]string{"user.name": "Bob"}})
	s.Require().Nil(err)
	s.Equal("Hi Bob", rc.Body)
}

// TestResolve_EscapesContextInHTMLBody verifies a context value is HTML-escaped in an email body but not
// in a plain SMS body.
func (s *RendererTestSuite) TestResolve_EscapesContextInHTMLBody() {
	data := map[string]string{"name": "<b>x</b>"}

	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "s", Body: "Hi {{ctx(name)}}"},
	})
	p := s.newRenderer(s.translator(nil), nil)
	email, err := p.Resolve(s.ctx, "email", "h", RenderInput{Data: data})
	s.Require().Nil(err)
	s.Equal("Hi &lt;b&gt;x&lt;/b&gt;", email.Body)

	s.expectGet(ChannelTypeSMS, "h2", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h2", DisplayName: "D",
		Content: TemplateContent{Body: "Hi {{ctx(name)}}"},
	})
	sms, err := p.Resolve(s.ctx, "sms", "h2", RenderInput{Data: data})
	s.Require().Nil(err)
	s.Equal("Hi <b>x</b>", sms.Body)
}

func (s *RendererTestSuite) TestResolve_MissingTranslation() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "t1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(absent.key)}}"},
	})
	p := s.newRenderer(s.translator(nil), nil)

	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	// A missing translation fails closed: the notification cannot be produced, surfaced as a 500.
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_TranslationError covers a non-not-found translation error, which also fails closed as 500.
func (s *RendererTestSuite) TestResolve_TranslationError() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "t1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(some.key)}}"},
	})
	tr := s.failingTranslator(&tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: "I18N-OTHER"})
	p := s.newRenderer(tr, nil)

	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_SubjectRenderFails covers a failing subject render (a missing translation in the subject),
// confirming the subject path fails closed before the body is rendered.
func (s *RendererTestSuite) TestResolve_SubjectRenderFails() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "{{t(absent.subject)}}", Body: "b"},
	})
	p := s.newRenderer(s.translator(nil), nil)

	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *RendererTestSuite) TestResolve_UnresolvedContextFails() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(sms.body)}}"},
	})
	p := s.newRenderer(s.translator(map[string]string{"sms.body": "Code {{ctx(otpCode)}}"}), nil)

	// No Data supplied, so {{ctx(otpCode)}} cannot be resolved -> fail closed, do not ship the token.
	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_UnresolvedDesignFails covers the design failure modes: no theme supplied, and a token the
// theme does not carry. Both fail closed rather than ship a partially-branded body.
func (s *RendererTestSuite) TestResolve_UnresolvedDesignFails() {
	emailTmpl := templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "s", Body: "{{design(palette.primary.main)}}"},
	}
	p := s.newRenderer(s.translator(nil), s.designer(nil))

	// No theme supplied: cannot resolve.
	s.expectGet(ChannelTypeEmail, "h", emailTmpl)
	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)

	// Theme supplied but the token is unknown: the resolver leaves it, so the render fails closed.
	s.expectGet(ChannelTypeEmail, "h", emailTmpl)
	_, err = p.Resolve(s.ctx, "email", "h", RenderInput{ThemeID: "theme1"})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_DesignResolverError covers the design resolver returning a service error.
func (s *RendererTestSuite) TestResolve_DesignResolverError() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "s", Body: "{{design(palette.primary.main)}}"},
	})
	p := s.newRenderer(s.translator(nil), s.failingDesigner())

	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{ThemeID: "theme1"})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_DesignTokenInSubjectFailsClosed covers a design token smuggled into the subject via a
// translation value: translated text may embed only {{ctx}}, so the render fails closed.
func (s *RendererTestSuite) TestResolve_DesignTokenInSubjectFailsClosed() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "{{t(subj)}}", Body: "static"},
	})
	tr := s.translator(map[string]string{"subj": "Hi {{design(palette.primary.main)}}"})
	p := s.newRenderer(tr, s.designer(map[string]string{"palette.primary.main": "#111"}))

	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{ThemeID: "theme1"})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_TranslationMemoizedAcrossSubjectAndBody verifies a key used in both the subject and the
// body (and repeated) is resolved by the translation service only once per render.
func (s *RendererTestSuite) TestResolve_TranslationMemoizedAcrossSubjectAndBody() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{
			Subject: "{{t(org.name)}}",
			Body:    "{{t(org.name)}} - {{t(org.name)}}",
		},
	})
	tr := s.translator(map[string]string{"org.name": "Acme"})
	p := s.newRenderer(tr, nil)

	rc, err := p.Resolve(s.ctx, "email", "h", RenderInput{Locale: "en-US"})
	s.Require().Nil(err)
	s.Equal("Acme", rc.Subject)
	s.Equal("Acme - Acme", rc.Body)
	tr.AssertNumberOfCalls(s.T(), "ResolveTranslationsForKey", 1)
}

// TestResolve_NonCanonicalLocaleNormalized verifies a non-canonical locale (en_US) is normalized to
// canonical BCP 47 (en-US) before the translation lookup, rather than being passed through verbatim.
func (s *RendererTestSuite) TestResolve_NonCanonicalLocaleNormalized() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(greeting)}}"},
	})
	tr := s.translator(map[string]string{"greeting": "Hi"})
	p := s.newRenderer(tr, nil)

	rc, err := p.Resolve(s.ctx, "sms", "h", RenderInput{Locale: "en_US"})
	s.Require().Nil(err)
	s.Equal("Hi", rc.Body)
	tr.AssertCalled(s.T(), "ResolveTranslationsForKey", mock.Anything, "en-US", mock.Anything, mock.Anything)
}

// TestResolve_NestedTranslationPlaceholderFailsClosed verifies a {{t()}} carried in by a translation
// value is rejected (translated text may embed only {{ctx}}).
func (s *RendererTestSuite) TestResolve_NestedTranslationPlaceholderFailsClosed() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(outer)}}"},
	})
	p := s.newRenderer(s.translator(map[string]string{"outer": "see {{t(inner)}}", "inner": "X"}), nil)

	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_DesignTokenInSMSFailsClosed verifies a design token carried into an SMS body via a
// translation value fails closed (translated text may embed only {{ctx}}).
func (s *RendererTestSuite) TestResolve_DesignTokenInSMSFailsClosed() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(body)}}"},
	})
	p := s.newRenderer(s.translator(map[string]string{"body": "color {{design(palette.primary.main)}}"}), nil)

	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_ContextValueWithPlaceholderTextNotRejected verifies a context value that itself contains
// placeholder-like text is substituted verbatim (the last pass) and not misread as an unresolved token.
func (s *RendererTestSuite) TestResolve_ContextValueWithPlaceholderTextNotRejected() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "Hi {{ctx(name)}}"},
	})
	p := s.newRenderer(s.translator(nil), nil)

	rc, err := p.Resolve(s.ctx, "sms", "h", RenderInput{Data: map[string]string{"name": "{{t(offer)}}"}})
	s.Require().Nil(err)
	s.Equal("Hi {{t(offer)}}", rc.Body)
}

// TestResolve_DesignTokenInBodyViaTranslationFailsClosed verifies a design token embedded in a
// translation value fails closed even in the email body (translated text may embed only {{ctx}}),
// consistent with the subject and SMS.
func (s *RendererTestSuite) TestResolve_DesignTokenInBodyViaTranslationFailsClosed() {
	s.expectGet(ChannelTypeEmail, "h", templateDAO{
		ID: "e1", Channel: ChannelTypeEmail, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Subject: "s", Body: "{{t(body)}}"},
	})
	tr := s.translator(map[string]string{"body": "color {{design(palette.primary.main)}}"})
	p := s.newRenderer(tr, s.designer(map[string]string{"palette.primary.main": "#111"}))

	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{ThemeID: "theme1"})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *RendererTestSuite) TestResolve_InvalidChannel() {
	p := s.newRenderer(s.translator(nil), nil)
	_, err := p.Resolve(s.ctx, "push", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidChannel.Code, err.Code)
}

func (s *RendererTestSuite) TestResolve_EmptyHandle() {
	p := s.newRenderer(s.translator(nil), nil)
	_, err := p.Resolve(s.ctx, "email", "", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(ErrorInvalidHandle.Code, err.Code)
}

func (s *RendererTestSuite) TestResolve_NotFound() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "missing").
		Return(templateDAO{}, errTemplateNotFound).Once()
	p := s.newRenderer(s.translator(nil), nil)

	_, err := p.Resolve(s.ctx, "email", "missing", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(ErrorTemplateNotFound.Code, err.Code)
}

// TestResolve_StoreError covers a non-not-found store error, surfaced as a 500.
func (s *RendererTestSuite) TestResolve_StoreError() {
	s.mockStore.On("GetTemplateByHandle", mock.Anything, ChannelTypeEmail, "h").
		Return(templateDAO{}, context.DeadlineExceeded).Once()
	p := s.newRenderer(s.translator(nil), nil)

	_, err := p.Resolve(s.ctx, "email", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// TestResolve_NoTranslationResolver verifies a render that reaches a {{t(...)}} key before the
// translation resolver is wired fails closed with a clean error instead of panicking on a nil resolver.
func (s *RendererTestSuite) TestResolve_NoTranslationResolver() {
	s.expectGet(ChannelTypeSMS, "h", templateDAO{
		ID: "s1", Channel: ChannelTypeSMS, Handle: "h", DisplayName: "D",
		Content: TemplateContent{Body: "{{t(sms.body)}}"},
	})
	// Renderer constructed with a nil translation resolver (defensive: a render then fails closed
	// instead of nil-panicking).
	p := newTemplateRenderer(s.mockStore, nil)

	_, err := p.Resolve(s.ctx, "sms", "h", RenderInput{})
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}
