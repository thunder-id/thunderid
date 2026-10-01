// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"fmt"
	"strings"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"

	"github.com/google/uuid"
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
	// Required. A declared gateway is held in memory for the life of the process and nothing about
	// it is written down between starts, so there is nowhere to keep a generated key: a fresh one
	// each start would stop the gateway answering until it was reconfigured. The file has to name
	// the key the gateway already holds.
	//
	// A literal key here is a gateway credential in source control, so write it as an environment
	// reference instead: `key: "{{.GATEWAY_TOKEN}}"` reads GATEWAY_TOKEN as the file is loaded, and
	// the file itself carries only the name.
	Key           string `yaml:"key"`
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
	// Refused here rather than loaded with nothing to present. A declared gateway holds its key in
	// memory only, so one the file does not name is one this control plane can never authenticate
	// to, on this start or any other.
	if strings.TrimSpace(declared.Key) == "" {
		return fmt.Errorf("gateway %q needs a key to authenticate with, such as "+
			"key: \"{{.GATEWAY_TOKEN}}\"", declared.Name)
	}
	return nil
}

// declaredGatewayStore registers what the loader reads.
//
// A declared gateway is identified by its name, so re-reading the same file updates the gateway it
// already registered rather than adding another. That is what makes the load idempotent, and a
// server reads these files on every start.
type declaredGatewayStore struct {
	store  *gatewayFileStore
	logger *log.Logger
}

func (d *declaredGatewayStore) Create(_ string, data interface{}) error {
	declared, ok := data.(*declaredGateway)
	if !ok {
		return fmt.Errorf("invalid data type: expected *declaredGateway")
	}

	// Declarative resources are read as the server starts, outside any request.
	ctx := context.Background()
	if err := d.store.put(Gateway{
		ID:            declaredGatewayID(declared.Name),
		Name:          declared.Name,
		BaseURL:       declared.BaseURL,
		CACertificate: declared.CACertificate,
		Key:           declared.Key,
	}); err != nil {
		return fmt.Errorf("failed to load gateway %q: %w", declared.Name, err)
	}
	d.logger.Info(ctx, "Loaded a declared gateway", log.String("gateway", declared.Name))
	return nil
}

// declaredGatewayID derives a gateway's id from its name.
//
// It has to be derived rather than generated, because nothing about a declared gateway is written
// down between starts: a fresh id each time would change the address of every route naming it, and
// anything holding one would be pointing at nothing after a restart. A name is unique among
// declared gateways, so a name-derived id is too. The namespace keeps it from colliding with an
// id derived the same way for some other resource.
func declaredGatewayID(name string) string {
	return uuid.NewSHA1(declaredGatewayNamespace, []byte(name)).String()
}

// declaredGatewayNamespace is the fixed UUIDv5 namespace declared gateway ids are derived in. It is
// arbitrary but must never change: changing it renames every declared gateway.
var declaredGatewayNamespace = uuid.MustParse("6f9619ff-8b86-d011-b42d-00c04fc964ff")

// loadDeclarativeGateways reads every gateway declared in a file into the in-memory store.
//
// Nothing here reaches the database. These files are read on every start and are the whole truth
// about what they declare, so a persisted row would outlive the file that made it.
func loadDeclarativeGateways(fileStore *gatewayFileStore) error {
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
	}, &declaredGatewayStore{store: fileStore, logger: logger})

	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load gateway resources: %w", err)
	}
	return nil
}
