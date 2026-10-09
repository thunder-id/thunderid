// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"context"
	"sort"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// resourceTypeNotificationTemplate names this resource in files and logs.
const resourceTypeNotificationTemplate = "NotificationTemplate"

// templateFileBasedStore holds file-declared templates in memory. Read-only; writes are refused.
type templateFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newFileBasedTemplateStore creates a file store over the shared declarative-resource store.
func newFileBasedTemplateStore() *templateFileBasedStore {
	return &templateFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStore(entity.KeyTypeNotificationTemplate),
	}
}

// newTestFileStore creates a file store over an isolated backing store.
func newTestFileStore() *templateFileBasedStore {
	return &templateFileBasedStore{
		GenericFileBasedStore: declarativeresource.NewGenericFileBasedStoreForTest(entity.KeyTypeNotificationTemplate),
	}
}

// put records a declared template; the loader's only way in.
func (f *templateFileBasedStore) put(t templateDAO) error {
	stored := t
	return f.GenericFileBasedStore.Create(t.ID, &stored)
}

// listAll returns every declared template across all channels.
func (f *templateFileBasedStore) listAll() ([]templateDAO, error) {
	entries, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}
	templates := make([]templateDAO, 0, len(entries))
	for _, item := range entries {
		t, ok := item.Data.(*templateDAO)
		if !ok {
			declarativeresource.LogTypeAssertionError(resourceTypeNotificationTemplate, "")
			continue
		}
		templates = append(templates, *t)
	}
	return templates, nil
}

// listByChannel returns a channel's declared templates, ordered by handle for stable pagination.
func (f *templateFileBasedStore) listByChannel(channel ChannelType) ([]templateDAO, error) {
	all, err := f.listAll()
	if err != nil {
		return nil, err
	}
	filtered := make([]templateDAO, 0, len(all))
	for _, t := range all {
		if t.Channel == channel {
			filtered = append(filtered, t)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Handle < filtered[j].Handle })
	return filtered, nil
}

// CreateTemplate refuses; files are the only writer.
func (f *templateFileBasedStore) CreateTemplate(_ context.Context, _ templateDAO) error {
	return errDeclarativeTemplate
}

// GetTemplate returns a declared template by channel and id.
func (f *templateFileBasedStore) GetTemplate(_ context.Context, channel ChannelType, id string) (templateDAO, error) {
	all, err := f.listAll()
	if err != nil {
		return templateDAO{}, err
	}
	for _, t := range all {
		if t.Channel == channel && t.ID == id {
			return t, nil
		}
	}
	return templateDAO{}, errTemplateNotFound
}

// GetTemplateByHandle returns a declared template by channel and handle.
func (f *templateFileBasedStore) GetTemplateByHandle(_ context.Context, channel ChannelType, handle string) (
	templateDAO, error) {
	all, err := f.listAll()
	if err != nil {
		return templateDAO{}, err
	}
	for _, t := range all {
		if t.Channel == channel && t.Handle == handle {
			return t, nil
		}
	}
	return templateDAO{}, errTemplateNotFound
}

// ListTemplates returns a page of declared templates of a channel.
func (f *templateFileBasedStore) ListTemplates(_ context.Context, channel ChannelType, limit, offset int) (
	[]templateDAO, error) {
	filtered, err := f.listByChannel(channel)
	if err != nil {
		return nil, err
	}
	if offset >= len(filtered) {
		return []templateDAO{}, nil
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[offset:end], nil
}

// CountTemplates returns the number of declared templates of a channel.
func (f *templateFileBasedStore) CountTemplates(_ context.Context, channel ChannelType) (int, error) {
	filtered, err := f.listByChannel(channel)
	if err != nil {
		return 0, err
	}
	return len(filtered), nil
}

// UpdateTemplate refuses; declared templates are read-only.
func (f *templateFileBasedStore) UpdateTemplate(_ context.Context, _ templateDAO) error {
	return errDeclarativeTemplate
}

// DeleteTemplate refuses; declared templates are read-only.
func (f *templateFileBasedStore) DeleteTemplate(_ context.Context, _ ChannelType, _ string) error {
	return errDeclarativeTemplate
}

// IsHandleExists reports whether a declared template in the channel already uses the handle.
func (f *templateFileBasedStore) IsHandleExists(_ context.Context, channel ChannelType, handle string) (bool, error) {
	all, err := f.listAll()
	if err != nil {
		return false, err
	}
	for _, t := range all {
		if t.Channel == channel && t.Handle == handle {
			return true, nil
		}
	}
	return false, nil
}
