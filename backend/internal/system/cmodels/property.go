// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package cmodels provides common data models used across server modules.
package cmodels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
)

// configCryptoProvider is injected once during application startup via
// SetConfigCryptoProvider. cmodels cannot import the kmprovider/defaultkm package directly
// to obtain it, since that package depends on cmodels and would create an import cycle.
var configCryptoProvider kmprovider.ConfigCryptoProvider

// SetConfigCryptoProvider sets the ConfigCryptoProvider used to encrypt and decrypt secret
// property values. It must be called once during application startup before any secret
// Property is created or read.
func SetConfigCryptoProvider(provider kmprovider.ConfigCryptoProvider) {
	configCryptoProvider = provider
}

// Property represents a generic property with name, value, and isSecret fields.
type Property struct {
	name     string
	value    string
	isSecret bool
}

// PropertyDTO represents a property for API communication.
type PropertyDTO struct {
	Name     string `json:"name"     yaml:"name"`
	Value    string `json:"value"    yaml:"value"`
	IsSecret bool   `json:"isSecret" yaml:"isSecret,omitempty"`
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

// GetValue returns the usable value of the property: decrypted if it is a secret, and resolved if it
// holds a reference to a secret held elsewhere.
func (p *Property) GetValue() (string, error) {
	value, err := p.UnresolvedValue()
	if err != nil {
		return "", err
	}
	if secretresolver.IsReference(value) {
		return p.resolveReference(value)
	}
	return value, nil
}

// UnresolvedValue returns the stored value, decrypted when it is a secret, but leaves a reference to
// a secret held elsewhere as it is.
//
// It is for callers that do not need the credential itself, such as checking a property is set or
// carrying it somewhere unchanged: resolving there would fail on a plane running no secret provider,
// and put the credential where it does not belong on one that does.
func (p *Property) UnresolvedValue() (string, error) {
	if secretresolver.IsReference(p.value) || !p.IsSecret() {
		return p.value, nil
	}

	if configCryptoProvider == nil {
		return "", errors.New("config crypto provider not initialized")
	}
	decryptedBytes, err := configCryptoProvider.Decrypt(context.Background(), []byte(p.value))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret property %s: %w", p.GetName(), err)
	}
	return string(decryptedBytes), nil
}

// resolveReference turns a secret reference into its value using the process-wide resolver.
func (p *Property) resolveReference(reference string) (string, error) {
	value, err := secretresolver.Default().Resolve(context.Background(), reference)
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret property %s: %w", p.GetName(), err)
	}
	return value, nil
}

// Encrypt encrypts the value if it's a secret property
func (p *Property) Encrypt() error {
	if !p.IsSecret() || p.value == "" {
		return nil
	}

	if configCryptoProvider == nil {
		return errors.New("config crypto provider not initialized")
	}
	encryptedBytes, err := configCryptoProvider.Encrypt(context.Background(), []byte(p.value))
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

// SerializePropertiesToJSONObject serializes an array of properties to a JSON object string
func SerializePropertiesToJSONObject(properties []Property) (string, error) {
	if len(properties) == 0 {
		return "", nil
	}

	type propertyEntry struct {
		Value    string `json:"value"`
		IsSecret bool   `json:"isSecret"`
	}

	obj := make(map[string]propertyEntry, len(properties))
	for _, property := range properties {
		obj[property.GetName()] = propertyEntry{
			Value:    property.value,
			IsSecret: property.IsSecret(),
		}
	}

	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("failed to serialize properties to JSON object: %w", err)
	}

	return string(jsonBytes), nil
}

// DeserializePropertiesFromJSONObject deserializes properties from a JSON object string
func DeserializePropertiesFromJSONObject(propertiesJSON string) ([]Property, error) {
	if propertiesJSON == "" {
		return []Property{}, nil
	}

	type propertyEntry struct {
		Value    string `json:"value"`
		IsSecret bool   `json:"isSecret"`
	}

	var obj map[string]propertyEntry
	if err := json.Unmarshal([]byte(propertiesJSON), &obj); err != nil {
		return nil, fmt.Errorf("failed to unmarshal properties JSON object: %w", err)
	}

	properties := make([]Property, 0, len(obj))
	for name, entry := range obj {
		property := Property{
			name:     name,
			value:    entry.Value,
			isSecret: entry.IsSecret,
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
	// Deliberately unresolved: this DTO is a transport shape, and a caller that serializes it would
	// otherwise put the credential itself in the payload. A runtime consumer that needs the value
	// asks for it through GetValue.
	value, err := p.UnresolvedValue()
	if err != nil {
		return nil, fmt.Errorf("failed to get value for property %s: %w", p.GetName(), err)
	}

	return &PropertyDTO{
		Name:     p.GetName(),
		Value:    value,
		IsSecret: p.IsSecret(),
	}, nil
}
