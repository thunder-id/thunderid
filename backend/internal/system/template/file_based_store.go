// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"context"
	"errors"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// templateFileBasedStore is a wrapper around GenericFileBasedStore that manages template resources.
type templateFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create stores a TemplateDTO in the underlying file-based store.
func (f *templateFileBasedStore) Create(id string, data interface{}) error {
	tmpl, ok := data.(*TemplateDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("template", id)
		return errors.New("invalid data type: expected *TemplateDTO")
	}
	return f.GenericFileBasedStore.Create(tmpl.ID, tmpl)
}

// GetTemplate retrieves a template by its ID.
func (f *templateFileBasedStore) GetTemplate(ctx context.Context, id string) (*TemplateDTO, error) {
	data, err := f.GenericFileBasedStore.Get(ctx, id)
	if err != nil {
		return nil, errTemplateNotFound
	}
	tmpl, ok := data.(*TemplateDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("template", id)
		return nil, errors.New("template data corrupted")
	}
	return tmpl, nil
}

// GetTemplateByScenario retrieves a template by its scenario type and template type.
func (f *templateFileBasedStore) GetTemplateByScenario(
	ctx context.Context, scenario ScenarioType, tmplType TemplateType,
) (*TemplateDTO, error) {
	compositeKey := string(scenario) + ":" + string(tmplType)
	data, err := f.GenericFileBasedStore.GetByField(ctx, compositeKey, func(d interface{}) string {
		if tmpl, ok := d.(*TemplateDTO); ok {
			return string(tmpl.Scenario) + ":" + string(tmpl.Type)
		}
		return ""
	})
	if err != nil {
		return nil, errTemplateNotFound
	}
	tmpl, ok := data.(*TemplateDTO)
	if !ok {
		declarativeresource.LogTypeAssertionError("template", "scenario:"+string(scenario)+":"+string(tmplType))
		return nil, errors.New("template data corrupted")
	}
	return tmpl, nil
}

// ListTemplates returns all templates stored in the file-based store.
func (f *templateFileBasedStore) ListTemplates(ctx context.Context) ([]*TemplateDTO, error) {
	list, err := f.GenericFileBasedStore.List(ctx)
	if err != nil {
		return nil, err
	}
	templates := make([]*TemplateDTO, 0, len(list))
	for _, item := range list {
		if tmpl, ok := item.Data.(*TemplateDTO); ok {
			templates = append(templates, tmpl)
		}
	}
	return templates, nil
}

// newTemplateFileBasedStore creates a new templateFileBasedStore using the underlying generic store.
func newTemplateFileBasedStore() *templateFileBasedStore {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeTemplate)
	return &templateFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}
