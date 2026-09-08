// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// resourceTypeGateway names this resource in declarative files and in errors.
const resourceTypeGateway = "Gateway"

// declaredGateway is the shape of a gateway declared in a file.
//
// It carries the same fields a registration takes through the API. The client secret is written in
// the file as it is, so the file needs the protection any file holding a credential needs; nothing
// here can make a readable file safe.
type declaredGateway struct {
	Name         string `yaml:"name"`
	BaseURL      string `yaml:"baseUrl"`
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
	Scope        string `yaml:"scope,omitempty"`
}

// parseDeclaredGateway reads one declared gateway.
func parseDeclaredGateway(data []byte) (interface{}, error) {
	var declared declaredGateway
	if err := yaml.Unmarshal(data, &declared); err != nil {
		return nil, fmt.Errorf("failed to parse the gateway: %w", err)
	}
	return &declared, nil
}

// validateDeclaredGateway refuses a declaration that could not be applied to.
func validateDeclaredGateway(data interface{}) error {
	declared, ok := data.(*declaredGateway)
	if !ok {
		declarativeresource.LogTypeAssertionError(resourceTypeGateway, "")
		return fmt.Errorf("invalid data type: expected *declaredGateway")
	}
	if strings.TrimSpace(declared.Name) == "" {
		return fmt.Errorf("a gateway needs a name")
	}
	if strings.TrimSpace(declared.BaseURL) == "" || strings.TrimSpace(declared.ClientID) == "" ||
		strings.TrimSpace(declared.ClientSecret) == "" {
		return fmt.Errorf("gateway %q needs a baseUrl, clientId and clientSecret to be reachable",
			declared.Name)
	}
	return nil
}

// declaredGatewayStore registers what the loader reads.
//
// A declared gateway is identified by its name, so re-reading the same file updates the gateway it
// already registered rather than adding another. That is what makes the load idempotent, and a
// server reads these files on every start.
type declaredGatewayStore struct {
	service ServiceInterface
	logger  *log.Logger
}

func (d *declaredGatewayStore) Create(_ string, data interface{}) error {
	declared, ok := data.(*declaredGateway)
	if !ok {
		return fmt.Errorf("invalid data type: expected *declaredGateway")
	}

	// Declarative resources are read as the server starts, outside any request.
	ctx := context.Background()
	svcErr := d.service.Adopt(ctx, RegisterRequest{
		Name:         declared.Name,
		BaseURL:      declared.BaseURL,
		ClientID:     declared.ClientID,
		ClientSecret: declared.ClientSecret,
		Scope:        declared.Scope,
	})
	if svcErr != nil {
		return fmt.Errorf("failed to register gateway %q: %s", declared.Name, svcErr.Error.DefaultValue)
	}
	d.logger.Info(ctx, "Registered a declared gateway", log.String("gateway", declared.Name))
	return nil
}

// loadDeclarativeGateways registers every gateway declared in a file.
func loadDeclarativeGateways(service ServiceInterface) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayDeclarative"))

	loader := declarativeresource.NewResourceLoader(declarativeresource.ResourceConfig{
		ResourceType:  resourceTypeGateway,
		DirectoryName: "gateways",
		Parser:        parseDeclaredGateway,
		Validator:     validateDeclaredGateway,
		IDExtractor: func(data interface{}) string {
			if declared, ok := data.(*declaredGateway); ok {
				return declared.Name
			}
			return ""
		},
	}, &declaredGatewayStore{service: service, logger: logger})

	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load gateway resources: %w", err)
	}
	return nil
}
