// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// memStore is an in-memory notificationTemplateStoreInterface for service tests.
type memStore struct {
	templates map[string]templateDAO
	// createErr, when set, is returned by CreateTemplate to simulate a store failure (e.g. a DB
	// unique-constraint violation from a concurrent insert).
	createErr error
}

func newMemStore() *memStore { return &memStore{templates: map[string]templateDAO{}} }

func (m *memStore) CreateTemplate(_ context.Context, t templateDAO) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.templates[t.ID] = t
	return nil
}

func (m *memStore) GetTemplate(_ context.Context, channel, id string) (templateDAO, error) {
	t, ok := m.templates[id]
	if !ok || t.Channel != channel {
		return templateDAO{}, errTemplateNotFound
	}
	return t, nil
}

func (m *memStore) ListTemplates(_ context.Context, channel string) ([]templateDAO, error) {
	var out []templateDAO
	for _, t := range m.templates {
		if t.Channel == channel {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *memStore) UpdateTemplate(_ context.Context, t templateDAO) error {
	m.templates[t.ID] = t
	return nil
}

func (m *memStore) DeleteTemplate(_ context.Context, channel, id string) error {
	if t, ok := m.templates[id]; ok && t.Channel == channel {
		delete(m.templates, id)
	}
	return nil
}

func (m *memStore) IsNameExists(_ context.Context, channel, name, excludeID string) (bool, error) {
	for id, t := range m.templates {
		if t.Channel == channel && t.Name == name && id != excludeID {
			return true, nil
		}
	}
	return false, nil
}

// inlineTransactioner runs the operation directly, mirroring a committed transaction.
type inlineTransactioner struct{}

func (inlineTransactioner) Transact(ctx context.Context, op func(context.Context) error) error {
	return op(ctx)
}

// stubRegistry is a minimal resourcedependency.Registry for delete tests.
type stubRegistry struct {
	resp *resourcedependency.DependenciesResponse
}

func (r *stubRegistry) RegisterProvider(resourcedependency.Provider) {}
func (r *stubRegistry) GetDependencies(context.Context, string, string) (
	*resourcedependency.DependenciesResponse, error) {
	return r.resp, nil
}
func (r *stubRegistry) CascadeDelete(context.Context, string, string) (int, error) { return 0, nil }
func (r *stubRegistry) ValidateReferenceUpdate(context.Context, string, string) *tidcommon.ServiceError {
	return nil
}

func newService() (NotificationTemplateServiceInterface, *memStore) {
	store := newMemStore()
	return newNotificationTemplateService(store, inlineTransactioner{}), store
}

func TestCreateTemplate_Email(t *testing.T) {
	svc, store := newService()
	ctx := context.Background()

	tmpl, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name:    "OTP Verification",
		Content: TemplateContent{Subject: "s.key", Body: "b.key"},
		Design:  &TemplateDesign{ColorScheme: ColorSchemeLight},
	})
	require.Nil(t, err)
	require.NotEmpty(t, tmpl.ID)
	require.Equal(t, ContentTypeHTML, tmpl.Content.ContentType) // defaulted
	require.Equal(t, "s.key", tmpl.Content.Subject)
	require.NotNil(t, tmpl.Design)
	require.Equal(t, ColorSchemeLight, tmpl.Design.ColorScheme)
	require.Equal(t, buildSelf(ChannelEmail, tmpl.ID), tmpl.Self)
	require.Len(t, store.templates, 1)
}

func TestCreateTemplate_SMSPlainBodySucceeds(t *testing.T) {
	svc, _ := newService()

	tmpl, err := svc.CreateTemplate(context.Background(), ChannelSMS, CreateTemplateRequest{
		Name:    "OTP Verification",
		Content: TemplateContent{Body: "b.key"},
	})
	require.Nil(t, err)
	require.Equal(t, ContentTypePlain, tmpl.Content.ContentType)
	require.Empty(t, tmpl.Content.Subject)
	require.Nil(t, tmpl.Design)
}

func TestCreateTemplate_SMSRejectsEmailOnlyFields(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateTemplate(ctx, ChannelSMS, CreateTemplateRequest{
		Name:    "OTP",
		Content: TemplateContent{Subject: "s.key", Body: "b.key"},
	})
	require.Equal(t, ErrorSubjectNotAllowed.Code, err.Code)

	_, err = svc.CreateTemplate(ctx, ChannelSMS, CreateTemplateRequest{
		Name:    "OTP",
		Content: TemplateContent{Body: "b.key"},
		Design:  &TemplateDesign{ColorScheme: ColorSchemeDark},
	})
	require.Equal(t, ErrorDesignNotAllowed.Code, err.Code)
}

func TestCreateTemplate_NameTooLong(t *testing.T) {
	svc, _ := newService()
	longName := make([]byte, maxNameLength+1)
	for i := range longName {
		longName[i] = 'a'
	}

	_, err := svc.CreateTemplate(context.Background(), ChannelEmail, CreateTemplateRequest{
		Name:    string(longName),
		Content: TemplateContent{Body: "b"},
	})
	require.Equal(t, ErrorNameTooLong.Code, err.Code)
}

func TestCreateTemplate_Validation(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateTemplate(ctx, "push", CreateTemplateRequest{Name: "n", Content: TemplateContent{Body: "b"}})
	require.Equal(t, ErrorInvalidChannel.Code, err.Code)

	_, err = svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{Content: TemplateContent{Body: "b"}})
	require.Equal(t, ErrorMissingName.Code, err.Code)

	_, err = svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{Name: "n"})
	require.Equal(t, ErrorMissingBodyKey.Code, err.Code)
}

func TestCreateTemplate_DescriptionTooLong(t *testing.T) {
	svc, _ := newService()
	longDesc := strings.Repeat("d", maxDescriptionLength+1)

	_, err := svc.CreateTemplate(context.Background(), ChannelEmail, CreateTemplateRequest{
		Name:        "OK",
		Description: longDesc,
		Content:     TemplateContent{Body: "b"},
	})
	require.Equal(t, ErrorDescriptionTooLong.Code, err.Code)
}

// TestCreateTemplate_ConcurrentUniqueViolation simulates the race where the name pre-check passes but
// the INSERT trips the DB UNIQUE constraint; the driver error must map to the 409 conflict, not a 500.
func TestCreateTemplate_ConcurrentUniqueViolation(t *testing.T) {
	svc, store := newService()
	store.createErr = errors.New("pq: duplicate key value violates unique constraint \"notification_template_deployment_id_channel_name_key\"")

	_, err := svc.CreateTemplate(context.Background(), ChannelEmail, CreateTemplateRequest{
		Name:    "Fresh",
		Content: TemplateContent{Body: "b"},
	})
	require.NotNil(t, err)
	require.Equal(t, ErrorTemplateNameConflict.Code, err.Code)
}

func TestIsUniqueViolation(t *testing.T) {
	require.False(t, isUniqueViolation(nil))
	require.False(t, isUniqueViolation(errors.New("some other error")))
	require.True(t, isUniqueViolation(errors.New("UNIQUE constraint failed: NOTIFICATION_TEMPLATE.NAME")))
	require.True(t, isUniqueViolation(fmt.Errorf("wrapped: %w",
		errors.New("pq: duplicate key value violates unique constraint"))))
}

func TestCreateTemplate_NameConflictPerChannel(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()
	req := CreateTemplateRequest{Name: "Dup", Content: TemplateContent{Body: "b"}}

	_, err := svc.CreateTemplate(ctx, ChannelEmail, req)
	require.Nil(t, err)

	// Same name, same channel -> conflict.
	_, err = svc.CreateTemplate(ctx, ChannelEmail, req)
	require.Equal(t, ErrorTemplateNameConflict.Code, err.Code)

	// Same name, different channel -> allowed (uniqueness is per channel).
	_, err = svc.CreateTemplate(ctx, ChannelSMS, req)
	require.Nil(t, err)
}

func TestUpdateTemplate(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	created, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "Original", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)

	// Updating its own record with the same name is not a conflict.
	updated, err := svc.UpdateTemplate(ctx, ChannelEmail, created.ID, UpdateTemplateRequest{
		Name: "Original", Description: "desc", Content: TemplateContent{Body: "b2"}})
	require.Nil(t, err)
	require.Equal(t, "desc", updated.Description)
	require.Equal(t, "b2", updated.Content.Body)

	// Updating a missing template -> not found.
	_, err = svc.UpdateTemplate(ctx, ChannelEmail, "missing", UpdateTemplateRequest{
		Name: "x", Content: TemplateContent{Body: "b"}})
	require.Equal(t, ErrorTemplateNotFound.Code, err.Code)
}

func TestUpdateTemplate_NameConflictWithOther(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	_, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "Taken", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)
	other, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "Other", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)

	_, err = svc.UpdateTemplate(ctx, ChannelEmail, other.ID, UpdateTemplateRequest{
		Name: "Taken", Content: TemplateContent{Body: "b"}})
	require.Equal(t, ErrorTemplateNameConflict.Code, err.Code)
}

func TestGetAndList(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	created, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "A", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)

	got, err := svc.GetTemplate(ctx, ChannelEmail, created.ID)
	require.Nil(t, err)
	require.Equal(t, created.ID, got.ID)

	_, err = svc.GetTemplate(ctx, ChannelEmail, "missing")
	require.Equal(t, ErrorTemplateNotFound.Code, err.Code)

	// A template created for one channel is not visible under another.
	_, err = svc.GetTemplate(ctx, ChannelSMS, created.ID)
	require.Equal(t, ErrorTemplateNotFound.Code, err.Code)

	list, err := svc.ListTemplates(ctx, ChannelEmail)
	require.Nil(t, err)
	require.Len(t, list.Templates, 1)
	require.Equal(t, created.ID, list.Templates[0].ID)
}

func TestDeleteTemplate(t *testing.T) {
	svc, store := newService()
	ctx := context.Background()

	// Idempotent: deleting an absent template succeeds.
	require.Nil(t, svc.DeleteTemplate(ctx, ChannelEmail, "missing"))

	created, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "A", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)

	require.Nil(t, svc.DeleteTemplate(ctx, ChannelEmail, created.ID))
	require.Len(t, store.templates, 0)
}

func TestDeleteTemplate_BlockedByFlowReference(t *testing.T) {
	svc, _ := newService()
	ctx := context.Background()

	created, err := svc.CreateTemplate(ctx, ChannelEmail, CreateTemplateRequest{
		Name: "A", Content: TemplateContent{Body: "b"}})
	require.Nil(t, err)

	total := 1
	svc.SetDependencyRegistry(&stubRegistry{resp: &resourcedependency.DependenciesResponse{
		TotalResults: &total,
		Usages: []resourcedependency.ResourceDependency{
			{BehaviorOnDelete: resourcedependency.BehaviorRestrict},
		},
	}})

	err = svc.DeleteTemplate(ctx, ChannelEmail, created.ID)
	require.Equal(t, ErrorTemplateInUse.Code, err.Code)
}
