// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strings"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// resourceTypeGateway names this resource in declarative files and in errors.
const resourceTypeGateway = "Gateway"

// declaredGateway is the shape of a gateway declared in a file.
//
// It carries the same fields a registration takes through the API.
type declaredGateway struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"baseUrl"`
	// Key is the token this control plane presents to that gateway.
	//
	// Optional, and what makes a gateway configured with a key first something this file can
	// bootstrap: declare the key it already holds and the two agree without anyone calling the API.
	// Omit it and the control plane issues one when it first adopts the file, keeping that one
	// across later reads.
	//
	// A literal key here is a gateway credential in source control, so write it as an
	// environment reference instead: `key: "{{.GATEWAY_TOKEN}}"` reads GATEWAY_TOKEN as the file is
	// loaded, and the file itself carries only the name.
	Key           string `yaml:"key,omitempty"`
	CACertificate string `yaml:"caCertificate,omitempty"`
}

// parseDeclaredGateway reads one declared gateway.
//
// Environment references are resolved before the document is read, so a declaration can name a
// credential it does not carry. A reference to something unset is an error rather than an empty
// key: a gateway registered with no token is one the control plane cannot present anything to.
func parseDeclaredGateway(data []byte) (interface{}, error) {
	resolved, err := sysutils.SubstituteEnvironmentVariables(data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve the gateway's environment references: %w", err)
	}

	var declared declaredGateway
	if err := yaml.Unmarshal(resolved, &declared); err != nil {
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
	if strings.TrimSpace(declared.BaseURL) == "" {
		return fmt.Errorf("gateway %q needs a baseUrl to be reachable", declared.Name)
	}
	// Refused as the file is read, rather than registering an address nothing can call. A declaration
	// is loaded at start-up, so the alternative is a server that comes up holding a gateway that can
	// never answer.
	if _, ok := normalizeBaseURL(declared.BaseURL); !ok {
		return fmt.Errorf("gateway %q needs an absolute http or https baseUrl, such as "+
			"https://dp.example.com:8090", declared.Name)
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
		Name:          declared.Name,
		BaseURL:       declared.BaseURL,
		CACertificate: declared.CACertificate,
		Key:           declared.Key,
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
