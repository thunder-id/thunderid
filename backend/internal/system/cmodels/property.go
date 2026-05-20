/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

// Package cmodels provides common data models used across server modules.
package cmodels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm"
)

// Property represents a generic property with name, value, and isSecret fields.
type Property struct {
	name     string
	value    string
	isSecret bool
}

// PropertyDTO represents a property for API communication.
type PropertyDTO struct {
	Name     string `json:"name" yaml:"name"`
	Value    string `json:"value" yaml:"value"`
	IsSecret bool   `json:"isSecret" yaml:"is_secret,omitempty"`
}

// NewProperty creates a new Property instance with the given parameters.
// If isSecret is true, the value will be automatically encrypted.
func NewProperty(name, value string, isSecret bool) (*Property, error) {
	property := &Property{
		name:     name,
		value:    value,
		isSecret: isSecret,
	}

	if isSecret && value != "" {
		if err := property.Encrypt(); err != nil {
			return nil, fmt.Errorf("failed to encrypt property %s: %w", name, err)
		}
	}

	return property, nil
}

// GetName returns the name of the property
func (p *Property) GetName() string {
	return p.name
}

// IsSecret returns whether the property is a secret
func (p *Property) IsSecret() bool {
	return p.isSecret
}

// GetValue returns the decrypted value if it's a secret, otherwise returns the plain value
func (p *Property) GetValue() (string, error) {
	if !p.IsSecret() {
		return p.value, nil
	}

	cryptoProvider, err := defaultkm.GetEncryptionService()
	if err != nil {
		return "", fmt.Errorf("failed to initialize encryption service: %w", err)
	}
	decryptedBytes, err := cryptoProvider.Decrypt(context.Background(), []byte(p.value))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret property %s: %w", p.GetName(), err)
	}

	return string(decryptedBytes), nil
}

// Encrypt encrypts the value if it's a secret property
func (p *Property) Encrypt() error {
	if !p.IsSecret() || p.value == "" {
		return nil
	}

	cryptoProvider, err := defaultkm.GetEncryptionService()
	if err != nil {
		return fmt.Errorf("failed to initialize encryption service: %w", err)
	}
	encryptedBytes, err := cryptoProvider.Encrypt(context.Background(), []byte(p.value))
	if err != nil {
		return fmt.Errorf("failed to encrypt secret property %s: %w", p.GetName(), err)
	}

	p.value = string(encryptedBytes)
	return nil
}

// SerializePropertiesToJSONArray serializes an array of properties to a JSON array string
func SerializePropertiesToJSONArray(properties []Property) (string, error) {
	if len(properties) == 0 {
		return "", nil
	}

	propertyDTOs := make([]PropertyDTO, 0, len(properties))
	for _, property := range properties {
		propertyDTO := PropertyDTO{
			Name:     property.GetName(),
			Value:    property.value,
			IsSecret: property.IsSecret(),
		}
		propertyDTOs = append(propertyDTOs, propertyDTO)
	}

	jsonBytes, err := json.Marshal(propertyDTOs)
	if err != nil {
		return "", fmt.Errorf("failed to serialize properties to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// DeserializePropertiesFromJSON deserializes an array of properties from JSON string
func DeserializePropertiesFromJSON(propertiesJSON string) ([]Property, error) {
	if propertiesJSON == "" {
		return []Property{}, nil
	}

	var propertyDTOs []PropertyDTO
	if err := json.Unmarshal([]byte(propertiesJSON), &propertyDTOs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties JSON: %w", err)
	}

	properties := make([]Property, 0, len(propertyDTOs))
	for _, propertyDTO := range propertyDTOs {
		property := Property{
			name:     propertyDTO.Name,
			value:    propertyDTO.Value,
			isSecret: propertyDTO.IsSecret,
		}
		properties = append(properties, property)
	}

	return properties, nil
}

// ToProperty converts PropertyDTO to Property.
func (dto *PropertyDTO) ToProperty() (*Property, error) {
	return NewProperty(dto.Name, dto.Value, dto.IsSecret)
}

// ToPropertyDTO converts Property to PropertyDTO.
func (p *Property) ToPropertyDTO() (*PropertyDTO, error) {
	value, err := p.GetValue()
	if err != nil {
		return nil, fmt.Errorf("failed to get value for property %s: %w", p.GetName(), err)
	}

	return &PropertyDTO{
		Name:     p.GetName(),
		Value:    value,
		IsSecret: p.IsSecret(),
	}, nil
}
