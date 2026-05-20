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
func (f *templateFileBasedStore) GetTemplate(_ context.Context, id string) (*TemplateDTO, error) {
	data, err := f.GenericFileBasedStore.Get(id)
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
	_ context.Context, scenario ScenarioType, tmplType TemplateType,
) (*TemplateDTO, error) {
	compositeKey := string(scenario) + ":" + string(tmplType)
	data, err := f.GenericFileBasedStore.GetByField(compositeKey, func(d interface{}) string {
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
func (f *templateFileBasedStore) ListTemplates(_ context.Context) ([]*TemplateDTO, error) {
	list, err := f.GenericFileBasedStore.List()
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
