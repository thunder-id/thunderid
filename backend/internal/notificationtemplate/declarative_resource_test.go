// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// declaredEmailYAML is a minimal valid email template declaration.
const declaredEmailYAML = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
description: Greeting
content:
  subject: Hi
  body: Hello {{ctx(name)}}
`

func loadDeclared(t *testing.T, fileStore *templateFileBasedStore, yaml string) {
	t.Helper()
	dto, err := parseDeclaredTemplate([]byte(yaml))
	if err != nil {
		t.Fatalf("parseDeclaredTemplate: %v", err)
	}
	if err := fileStore.Create("", dto); err != nil {
		t.Fatalf("fileStore.Create: %v", err)
	}
}

type DeclarativeResourceTestSuite struct {
	suite.Suite
}

func TestDeclarativeResourceTestSuite(t *testing.T) {
	suite.Run(t, new(DeclarativeResourceTestSuite))
}

func (s *DeclarativeResourceTestSuite) TestValidationRejectsBadDoc() {
	// SMS with a subject is invalid.
	const badSMS = `resource_type: notification_template
id: otp-sms
channel: sms
handle: otp
ouId: 01900000-0000-7000-8000-000000000001
displayName: OTP
content:
  subject: nope
  body: Code {{ctx(code)}}
`
	dto, err := parseDeclaredTemplate([]byte(badSMS))
	s.Require().NoError(err)
	validate := newDeclaredTemplateValidator(nil)
	s.Require().Error(validate(dto), "expected validation to reject an SMS template with a subject")
}

func (s *DeclarativeResourceTestSuite) TestValidatorRejectsMissingOU() {
	const noOU = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
displayName: Welcome
content:
  subject: Hi
  body: Hello
`
	dto, err := parseDeclaredTemplate([]byte(noOU))
	s.Require().NoError(err)
	validate := newDeclaredTemplateValidator(nil)
	s.Require().ErrorContains(validate(dto), "missing the required ouId")
}

func (s *DeclarativeResourceTestSuite) TestValidatorRejectsDuplicateID() {
	const first = `resource_type: notification_template
id: dup-id
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
content:
  subject: Hi
  body: Hello
`
	const sameID = `resource_type: notification_template
id: dup-id
channel: email
handle: goodbye
ouId: 01900000-0000-7000-8000-000000000001
displayName: Goodbye
content:
  subject: Bye
  body: Bye
`
	validate := newDeclaredTemplateValidator(nil)
	firstDTO, err := parseDeclaredTemplate([]byte(first))
	s.Require().NoError(err)
	s.Require().NoError(validate(firstDTO))

	dupDTO, err := parseDeclaredTemplate([]byte(sameID))
	s.Require().NoError(err)
	s.Require().ErrorContains(validate(dupDTO), "duplicate declared notification template id")
}

func (s *DeclarativeResourceTestSuite) TestValidatorRejectsDuplicateChannelHandle() {
	const first = `resource_type: notification_template
id: id-1
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
content:
  subject: Hi
  body: Hello
`
	const sameHandle = `resource_type: notification_template
id: id-2
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome Again
content:
  subject: Hi
  body: Hello
`
	validate := newDeclaredTemplateValidator(nil)
	firstDTO, err := parseDeclaredTemplate([]byte(first))
	s.Require().NoError(err)
	s.Require().NoError(validate(firstDTO))

	dupDTO, err := parseDeclaredTemplate([]byte(sameHandle))
	s.Require().NoError(err)
	s.Require().ErrorContains(validate(dupDTO), "duplicate declared notification template for channel")
}

func (s *DeclarativeResourceTestSuite) TestParseRejectsMalformedYAML() {
	_, err := parseDeclaredTemplate([]byte("channel: email\n  bad: ["))
	s.Require().Error(err)
}

// declaredForDBCheck is a valid email declaration used by the composite-mode DB-collision tests.
const declaredForDBCheck = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
content:
  subject: Hi
  body: Hello
`

// TestValidatorRejectsHandleTakenByDB verifies that in composite mode a declared handle already used
// by a database template fails the load.
func (s *DeclarativeResourceTestSuite) TestValidatorRejectsHandleTakenByDB() {
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "welcome").Return(true, nil)

	dto, err := parseDeclaredTemplate([]byte(declaredForDBCheck))
	s.Require().NoError(err)
	err = newDeclaredTemplateValidator(db)(dto)
	s.Require().ErrorContains(err, "already used by a")
}

// TestValidatorRejectsIDTakenByDB verifies that a declared id already used by a database template
// fails the load even when the handle is free.
func (s *DeclarativeResourceTestSuite) TestValidatorRejectsIDTakenByDB() {
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "welcome").Return(false, nil)
	db.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome-email").
		Return(emailDAO("welcome-email", "DB"), nil)

	dto, err := parseDeclaredTemplate([]byte(declaredForDBCheck))
	s.Require().NoError(err)
	err = newDeclaredTemplateValidator(db)(dto)
	s.Require().ErrorContains(err, "already used by a database template")
}

// TestValidatorPassesWhenFreeInDB verifies the DB-collision check passes when neither the handle nor
// the id is taken (the GetTemplate lookup returning not-found is the clean path).
func (s *DeclarativeResourceTestSuite) TestValidatorPassesWhenFreeInDB() {
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "welcome").Return(false, nil)
	db.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome-email").
		Return(templateDAO{}, errTemplateNotFound)

	dto, err := parseDeclaredTemplate([]byte(declaredForDBCheck))
	s.Require().NoError(err)
	s.Require().NoError(newDeclaredTemplateValidator(db)(dto))
}

// TestValidatorSurfacesDBError verifies a transient database error during the handle check is
// surfaced rather than treated as "not taken".
func (s *DeclarativeResourceTestSuite) TestValidatorSurfacesDBError() {
	db := newNotificationTemplateStoreInterfaceMock(s.T())
	db.On("IsHandleExists", mock.Anything, ChannelTypeEmail, "welcome").
		Return(false, errors.New("db down"))

	dto, err := parseDeclaredTemplate([]byte(declaredForDBCheck))
	s.Require().NoError(err)
	s.Require().ErrorContains(newDeclaredTemplateValidator(db)(dto), "failed to check the database")
}

// TestLoadDeclarativeTemplatesReadsFromDisk drives loadDeclarativeTemplates end-to-end: it writes a
// declaration into the server's config/resources/notification_templates directory and verifies the
// loader parses, validates, and stores it in the file store.
func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeTemplatesReadsFromDisk() {
	serverHome := s.T().TempDir()
	dir := filepath.Join(serverHome, "config", "resources", "notification_templates")
	s.Require().NoError(os.MkdirAll(dir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "welcome.yaml"), []byte(declaredForDBCheck), 0600))

	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(serverHome,
		&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: true}}))
	defer config.ResetServerRuntime()

	fileStore := newTestFileStore()
	s.Require().NoError(loadDeclarativeTemplates(fileStore, nil))

	got, err := fileStore.GetTemplateByHandle(context.Background(), ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().Equal("welcome-email", got.ID)
	s.Require().Equal("01900000-0000-7000-8000-000000000001", got.OUID)
	s.Require().Equal("Hello", got.Content.Body)
}

// TestLoadDeclarativeTemplatesRejectsInvalidDoc verifies a malformed declaration on disk fails the
// load with a wrapped error.
func (s *DeclarativeResourceTestSuite) TestLoadDeclarativeTemplatesRejectsInvalidDoc() {
	const badSMS = `resource_type: notification_template
id: otp-sms
channel: sms
handle: otp
ouId: 01900000-0000-7000-8000-000000000001
displayName: OTP
content:
  subject: nope
  body: Code
`
	serverHome := s.T().TempDir()
	dir := filepath.Join(serverHome, "config", "resources", "notification_templates")
	s.Require().NoError(os.MkdirAll(dir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "otp.yaml"), []byte(badSMS), 0600))

	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(serverHome,
		&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: true}}))
	defer config.ResetServerRuntime()

	err := loadDeclarativeTemplates(newTestFileStore(), nil)
	s.Require().ErrorContains(err, "failed to load notification template resources")
}

// loadTemplateFile writes one declaration into a temp server home and runs the loader against it.
func (s *DeclarativeResourceTestSuite) loadTemplateFile(yaml string) (*templateFileBasedStore, error) {
	serverHome := s.T().TempDir()
	dir := filepath.Join(serverHome, "config", "resources", "notification_templates")
	s.Require().NoError(os.MkdirAll(dir, 0750))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "tpl.yaml"), []byte(yaml), 0600))

	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(serverHome,
		&config.Config{DeclarativeResources: config.DeclarativeResources{Enabled: true}}))
	s.T().Cleanup(config.ResetServerRuntime)

	fileStore := newTestFileStore()
	return fileStore, loadDeclarativeTemplates(fileStore, nil)
}

// TestLoadResolvesEnvVarAndKeepsPlaceholders verifies the loader resolves a {{.VAR}} env reference while
// leaving runtime placeholders like {{ctx(name)}} and {{design(...)}} intact.
func (s *DeclarativeResourceTestSuite) TestLoadResolvesEnvVarAndKeepsPlaceholders() {
	s.T().Setenv("WELCOME_SUBJECT", "Welcome aboard")
	const tpl = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
content:
  subject: "{{.WELCOME_SUBJECT}}"
  body: "Hi {{ctx(name)}} <span style=\"color:{{design(palette.primary.main)}}\">!</span>"
`
	fileStore, err := s.loadTemplateFile(tpl)
	s.Require().NoError(err)

	got, err := fileStore.GetTemplateByHandle(context.Background(), ChannelTypeEmail, "welcome")
	s.Require().NoError(err)
	s.Require().Equal("Welcome aboard", got.Content.Subject)
	expectedBody := "Hi {{ctx(name)}} " +
		"<span style=\"color:{{design(palette.primary.main)}}\">!</span>"
	s.Require().Equal(expectedBody, got.Content.Body)
}

// TestLoadFailsOnMissingEnvVar verifies an unresolved {{.VAR}} env reference fails the load.
func (s *DeclarativeResourceTestSuite) TestLoadFailsOnMissingEnvVar() {
	const tpl = `resource_type: notification_template
id: welcome-email
channel: email
handle: welcome
ouId: 01900000-0000-7000-8000-000000000001
displayName: Welcome
content:
  subject: "{{.MISSING_ENV_VAR}}"
  body: Hello
`
	_, err := s.loadTemplateFile(tpl)
	s.Require().Error(err)
}

// TestLoadAcceptsRuntimePlaceholders verifies runtime placeholders pass channel validation at load.
func (s *DeclarativeResourceTestSuite) TestLoadAcceptsRuntimePlaceholders() {
	const tpl = `resource_type: notification_template
id: otp-email
channel: email
handle: otp
ouId: 01900000-0000-7000-8000-000000000001
displayName: OTP
content:
  subject: "Your code"
  body: "{{ctx(otp)}} <span style=\"color:{{design(palette.primary.main)}}\">expires soon</span>"
`
	fileStore, err := s.loadTemplateFile(tpl)
	s.Require().NoError(err)

	got, err := fileStore.GetTemplateByHandle(context.Background(), ChannelTypeEmail, "otp")
	s.Require().NoError(err)
	s.Require().Equal("otp-email", got.ID)
}

type ExporterTestSuite struct {
	suite.Suite
	ctx      context.Context
	service  *NotificationTemplateServiceInterfaceMock
	exporter declarativeresource.ResourceExporter
}

func TestExporterTestSuite(t *testing.T) {
	suite.Run(t, new(ExporterTestSuite))
}

func (s *ExporterTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.service = NewNotificationTemplateServiceInterfaceMock(s.T())
	s.exporter = newNotificationTemplateExporter(s.service)
}

func (s *ExporterTestSuite) TestGetResourceType() {
	assert.Equal(s.T(), "notification_template", s.exporter.GetResourceType())
}

func (s *ExporterTestSuite) TestGetParameterizerType() {
	assert.Equal(s.T(), "NotificationTemplate", s.exporter.GetParameterizerType())
}

func (s *ExporterTestSuite) TestGetAllResourceIDs() {
	s.service.On("ListTemplates", mock.Anything, ChannelTypeEmail, serverconst.MaxPageSize, 0).
		Return(&TemplateListResponse{Templates: []TemplateSummary{
			{ID: "welcome-email"}, {ID: "reset-email"}, {ID: "declared-email"},
		}}, nil)
	s.service.On("ListTemplates", mock.Anything, ChannelTypeSMS, serverconst.MaxPageSize, 0).
		Return(&TemplateListResponse{Templates: []TemplateSummary{{ID: "otp-sms"}}}, nil)
	// The summary omits isReadOnly, so each template is fetched to decide; the declared one is skipped.
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome-email").
		Return(&Template{ID: "welcome-email"}, nil)
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "reset-email").
		Return(&Template{ID: "reset-email"}, nil)
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "declared-email").
		Return(&Template{ID: "declared-email", IsReadOnly: true}, nil)
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "otp-sms").
		Return(&Template{ID: "otp-sms"}, nil)

	ids, err := s.exporter.GetAllResourceIDs(s.ctx)
	s.Require().Nil(err)
	assert.ElementsMatch(s.T(), []string{"welcome-email", "reset-email", "otp-sms"}, ids)
}

// TestExportMultilineBodyRoundTrips drives the real exporter through the real export service, then
// loads the exported YAML back through this package's parser. It guards the GetResourceRules()==nil
// contract: a multi-line body containing ": " must survive export as valid YAML and re-parse intact.
func (s *ExporterTestSuite) TestExportMultilineBodyRoundTrips() {
	const body = "Hello {{ctx(name)}}\nNote: your code is {{ctx(otp)}}\nThanks"
	tmpl := &Template{
		ID: "welcome-email", Handle: "welcome", DisplayName: "Welcome",
		Content: TemplateContent{Subject: "Subject: your code", Body: body},
	}
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome-email").Return(tmpl, nil)
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "welcome-email").
		Return(nil, &ErrorTemplateNotFound).Maybe()

	exportSvc := export.Initialize(http.NewServeMux(),
		[]declarativeresource.ResourceExporter{s.exporter}, export.TemplatePlaceholders)
	result, svcErr := exportSvc.ExportResources(s.ctx,
		&export.ExportRequest{NotificationTemplates: []string{"welcome-email"}})
	s.Require().Nil(svcErr)
	s.Require().Len(result.Files, 1)

	// Load the exported document back through the declarative parser: it must parse and keep the body.
	parsed, err := parseDeclaredTemplate([]byte(result.Files[0].Content))
	s.Require().NoError(err)
	declared, ok := parsed.(*declaredNotificationTemplate)
	s.Require().True(ok)
	assert.Equal(s.T(), body, declared.Content.Body)
}

func (s *ExporterTestSuite) TestGetAllResourceIDs_ListError() {
	// The channel sweep order is unspecified, so either channel may be the one listed first and error.
	s.service.On("ListTemplates", mock.Anything, ChannelTypeEmail, serverconst.MaxPageSize, 0).
		Return(nil, &tidcommon.InternalServerError).Maybe()
	s.service.On("ListTemplates", mock.Anything, ChannelTypeSMS, serverconst.MaxPageSize, 0).
		Return(nil, &tidcommon.InternalServerError).Maybe()

	ids, err := s.exporter.GetAllResourceIDs(s.ctx)
	s.Require().NotNil(err)
	assert.Nil(s.T(), ids)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (s *ExporterTestSuite) TestGetAllResourceIDs_GetTemplateError() {
	s.service.On("ListTemplates", mock.Anything, ChannelTypeEmail, serverconst.MaxPageSize, 0).
		Return(&TemplateListResponse{Templates: []TemplateSummary{{ID: "welcome-email"}}}, nil).Maybe()
	s.service.On("ListTemplates", mock.Anything, ChannelTypeSMS, serverconst.MaxPageSize, 0).
		Return(&TemplateListResponse{Templates: []TemplateSummary{{ID: "otp-sms"}}}, nil).Maybe()
	// The per-id read-only fetch fails; the error propagates.
	s.service.On("GetTemplate", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.InternalServerError)

	_, err := s.exporter.GetAllResourceIDs(s.ctx)
	s.Require().NotNil(err)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (s *ExporterTestSuite) TestGetResourceByID() {
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome-email").
		Return(&Template{ID: "welcome-email", Handle: "welcome", DisplayName: "Welcome",
			Content: TemplateContent{Subject: "Hi", Body: "Hello"}}, nil)
	// Sweep order is unspecified; the other channel is queried only if visited before the match.
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "welcome-email").
		Return(nil, &ErrorTemplateNotFound).Maybe()

	resource, name, err := s.exporter.GetResourceByID(s.ctx, "welcome-email")
	s.Require().Nil(err)
	assert.Equal(s.T(), "email-welcome", name)
	declared, ok := resource.(*declaredNotificationTemplate)
	s.Require().True(ok)
	assert.Equal(s.T(), "welcome-email", declared.ID)
	assert.Equal(s.T(), "email", declared.Channel)
	assert.Equal(s.T(), "Hello", declared.Content.Body)
}

func (s *ExporterTestSuite) TestGetResourceByID_SweepsPastNonMatchingChannel() {
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "otp-sms").
		Return(&Template{ID: "otp-sms", Handle: "otp", Content: TemplateContent{Body: "Code"}}, nil)
	// The sweep continues past any channel that reports not-found; that channel is queried only if
	// visited before the match, so the order is unspecified.
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "otp-sms").
		Return(nil, &ErrorTemplateNotFound).Maybe()

	resource, name, err := s.exporter.GetResourceByID(s.ctx, "otp-sms")
	s.Require().Nil(err)
	assert.Equal(s.T(), "sms-otp", name)
	assert.NotNil(s.T(), resource)
}

func (s *ExporterTestSuite) TestGetResourceByID_NotFound() {
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "missing").
		Return(nil, &ErrorTemplateNotFound)
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "missing").
		Return(nil, &ErrorTemplateNotFound)

	_, _, err := s.exporter.GetResourceByID(s.ctx, "missing")
	s.Require().NotNil(err)
	assert.Equal(s.T(), ErrorTemplateNotFound.Code, err.Code)
}

func (s *ExporterTestSuite) TestGetResourceByID_ServiceError() {
	// The sweep returns on the first non-not-found error; either channel may be visited first.
	s.service.On("GetTemplate", mock.Anything, ChannelTypeEmail, "welcome").
		Return(nil, &tidcommon.InternalServerError).Maybe()
	s.service.On("GetTemplate", mock.Anything, ChannelTypeSMS, "welcome").
		Return(nil, &tidcommon.InternalServerError).Maybe()

	_, _, err := s.exporter.GetResourceByID(s.ctx, "welcome")
	s.Require().NotNil(err)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, err.Code)
}

func (s *ExporterTestSuite) TestValidateResource() {
	declared := &declaredNotificationTemplate{ID: "welcome-email", Channel: "email", Handle: "welcome"}
	name, err := s.exporter.ValidateResource(s.ctx, declared, "welcome-email", nil)
	assert.Nil(s.T(), err)
	assert.Equal(s.T(), "email-welcome", name)
}

func (s *ExporterTestSuite) TestValidateResource_InvalidType() {
	name, err := s.exporter.ValidateResource(s.ctx, "nope", "welcome-email", nil)
	s.Require().NotNil(err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_TYPE", err.Code)
}

func (s *ExporterTestSuite) TestValidateResource_MissingHandle() {
	declared := &declaredNotificationTemplate{ID: "welcome-email", Channel: "email"}
	name, err := s.exporter.ValidateResource(s.ctx, declared, "welcome-email", log.GetLogger())
	s.Require().NotNil(err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_NOTIFICATION_TEMPLATE", err.Code)
}

func (s *ExporterTestSuite) TestValidateResource_InvalidChannel() {
	declared := &declaredNotificationTemplate{ID: "welcome-email", Channel: "", Handle: "welcome"}
	name, err := s.exporter.ValidateResource(s.ctx, declared, "welcome-email", log.GetLogger())
	s.Require().NotNil(err)
	assert.Empty(s.T(), name)
	assert.Equal(s.T(), "INVALID_NOTIFICATION_TEMPLATE", err.Code)
}

func (s *ExporterTestSuite) TestGetResourceRules() {
	// Nil rules keep the parameterizer from quoting scalars (which would corrupt multi-line bodies).
	rules := s.exporter.GetResourceRules()
	s.Require().Nil(rules)
}
