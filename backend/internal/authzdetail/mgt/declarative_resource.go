// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	resourceTypeAuthorizationDetailType = "authorization_detail_type"
	paramTypeAuthorizationDetailType    = "AuthorizationDetailType"
)

// typeExporter implements declarativeresource.ResourceExporter for authorization details types.
type typeExporter struct {
	service AuthorizationDetailMgtServiceInterface
}

// newTypeExporter creates a new authorization details type exporter.
func newTypeExporter(service AuthorizationDetailMgtServiceInterface) *typeExporter {
	return &typeExporter{service: service}
}

// NewTypeExporterForTest creates an authorization details type exporter for tests.
func NewTypeExporterForTest(service AuthorizationDetailMgtServiceInterface) declarativeresource.ResourceExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newTypeExporter(service)
}

// GetResourceType returns the resource type for authorization details types.
func (e *typeExporter) GetResourceType() string {
	return resourceTypeAuthorizationDetailType
}

// GetParameterizerType returns the parameterizer type for authorization details types.
func (e *typeExporter) GetParameterizerType() string {
	return paramTypeAuthorizationDetailType
}

// GetAllResourceIDs retrieves the ids of every type.
// In composite mode, this excludes declarative (YAML-based) types.
func (e *typeExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	types, svcErr := e.service.ListAllAuthorizationDetailTypes(ctx)
	if svcErr != nil {
		return nil, svcErr
	}
	ids := make([]string, 0, len(types))
	for _, t := range types {
		isDeclarative, svcErr := e.service.IsAuthorizationDetailTypeDeclarative(ctx, t.ID)
		if svcErr != nil {
			return nil, svcErr
		}
		if !isDeclarative {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

// GetResourceByID retrieves a type by its id, in its declarative form.
func (e *typeExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError) {
	t, svcErr := e.service.GetAuthorizationDetailType(ctx, id)
	if svcErr != nil {
		return nil, "", svcErr
	}
	resource, err := declarativeResourceOf(t)
	if err != nil {
		e.logger().Error(ctx, "Failed to export an authorization details type", log.String("id", id),
			log.Error(err))
		return nil, "", &tidcommon.InternalServerError
	}
	return resource, t.Name, nil
}

// declarativeResourceOf returns a type in its declarative form, with its processor credential in clear for
// export.
func declarativeResourceOf(t *providers.AuthorizationDetailType) (*typeDeclarativeResource, error) {
	var schema bytes.Buffer
	if err := json.Compact(&schema, t.Schema); err != nil {
		return nil, fmt.Errorf("the schema is not valid JSON: %w", err)
	}
	processor, err := declarativeProcessorOf(t.Processor)
	if err != nil {
		return nil, fmt.Errorf("the processor authentication cannot be read: %w", err)
	}
	reuse := t.Reuse
	return &typeDeclarativeResource{
		ID:              t.ID,
		Type:            t.Type,
		Name:            t.Name,
		Description:     t.Description,
		Schema:          schemaText(schema.Bytes()),
		ConsentTemplate: t.ConsentTemplate,
		Processor:       processor,
		Reuse:           &reuse,
	}, nil
}

// declarativeProcessorOf returns a processor in its declarative form, with credentials in clear for export.
func declarativeProcessorOf(processor *providers.AuthorizationDetailProcessor) (*declarativeProcessor, error) {
	if processor == nil {
		return nil, nil
	}
	config, err := outboundauth.FromProperties(processor.AuthenticationProperties)
	if err != nil {
		return nil, err
	}
	// none is written rather than left out, since an import without the block keeps the stored credential.
	authentication := &outboundauth.Authentication{Type: string(outboundauth.TypeNone)}
	if config.Enabled() {
		authentication = &outboundauth.Authentication{Type: string(config.Type), Properties: config.Properties}
	}
	return &declarativeProcessor{Endpoint: processor.Endpoint, Authentication: authentication}, nil
}

// logger returns the logger of the authorization details type exporter.
func (e *typeExporter) logger() *log.Logger {
	return log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AuthorizationDetailTypeExporter"))
}

// ValidateResource validates an authorization details type resource.
func (e *typeExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	t, ok := resource.(*typeDeclarativeResource)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeAuthorizationDetailType, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		t.Name, resourceTypeAuthorizationDetailType, id, "ADT_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return t.Name, nil
}

// GetResourceRules returns the parameterization rules for authorization details types.
func (e *typeExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return nil
}

// GetResourceRulesForResource externalizes each processor credential of a type to a template variable.
func (e *typeExporter) GetResourceRulesForResource(resource interface{}) *declarativeresource.ResourceRules {
	t, ok := resource.(*typeDeclarativeResource)
	if !ok || t.Processor == nil {
		return nil
	}
	variables := outboundauth.SecretVariablePaths(t.Processor.Authentication, "Processor.Authentication.Properties")
	if len(variables) == 0 {
		return nil
	}
	return &declarativeresource.ResourceRules{SecretVariables: variables}
}

// SecretFields names the fields of an exported type that hold a processor credential.
func (e *typeExporter) SecretFields() []string {
	return outboundauth.SecretFieldPaths("processor.authentication.properties")
}

// ViewResource shows an exported type as the management API returns it, with its processor credential
// masked.
func (e *typeExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	t, err := ParseAuthorizationDetailTypeNode(document)
	if err != nil {
		return nil, err
	}
	return toTypeResponse(*t), nil
}

// typeDeclarativeResource is the declarative YAML shape of a type.
type typeDeclarativeResource struct {
	ID              string                `yaml:"id"`
	Type            string                `yaml:"type"`
	Name            string                `yaml:"name"`
	Description     string                `yaml:"description,omitempty"`
	Schema          interface{}           `yaml:"schema"`
	ConsentTemplate string                `yaml:"consentTemplate,omitempty"`
	Processor       *declarativeProcessor `yaml:"processor,omitempty"`
	Reuse           *bool                 `yaml:"reuse,omitempty"`
}

// declarativeProcessor is the declarative form of a type's processor and its outbound authentication.
type declarativeProcessor struct {
	Endpoint       string                       `yaml:"endpoint"`
	Authentication *outboundauth.Authentication `yaml:"authentication,omitempty"`
}

// schemaText exports a schema as its registered JSON text to keep the property order the consent screen uses.
type schemaText json.RawMessage

// MarshalJSON returns the schema's JSON text.
func (s schemaText) MarshalJSON() ([]byte, error) {
	return s, nil
}

// MarshalYAML returns the schema's JSON text, to be written as a string.
func (s schemaText) MarshalYAML() (interface{}, error) {
	return string(s), nil
}

// ParseAuthorizationDetailTypeNode reads an authorization details type from its declarative YAML form.
func ParseAuthorizationDetailTypeNode(node *yaml.Node) (*providers.AuthorizationDetailType, error) {
	var doc typeDeclarativeResource
	if err := node.Decode(&doc); err != nil {
		return nil, fmt.Errorf("failed to parse authorization details type YAML: %w", err)
	}
	if len(doc.ID) > maxTypeIDLength {
		return nil, fmt.Errorf("invalid id for authorization details type '%s': the id must be at most %d "+
			"characters", doc.Type, maxTypeIDLength)
	}
	var schema json.RawMessage
	switch v := doc.Schema.(type) {
	case nil:
	case string:
		if !json.Valid([]byte(v)) {
			return nil, fmt.Errorf(
				"invalid schema for authorization details type '%s': the schema is not valid JSON",
				doc.Type)
		}
		schema = json.RawMessage(v)
	default:
		// Convert from the node, since a decoded mapping loses the schema's property order.
		var schemaDoc struct {
			Schema yaml.Node `yaml:"schema"`
		}
		if err := node.Decode(&schemaDoc); err != nil {
			return nil, fmt.Errorf("invalid schema for authorization details type '%s': %w", doc.Type, err)
		}
		var encoded bytes.Buffer
		if err := writeYAMLNodeAsJSON(&encoded, &schemaDoc.Schema); err != nil {
			return nil, fmt.Errorf("invalid schema for authorization details type '%s': %w", doc.Type, err)
		}
		schema = encoded.Bytes()
	}
	var processor *providers.AuthorizationDetailProcessor
	var authentication *outboundauth.Authentication
	if doc.Processor != nil {
		processor = &providers.AuthorizationDetailProcessor{Endpoint: doc.Processor.Endpoint}
		authentication = doc.Processor.Authentication
	}
	// The credential is carried as written; the service, or the declarative loader, resolves it.
	properties, svcErr := sentAuthenticationProperties(authentication)
	if svcErr != nil {
		return nil, fmt.Errorf("invalid processor authentication for authorization details type '%s': %s",
			doc.Type, svcErr.ErrorDescription.String())
	}
	if processor != nil {
		processor.AuthenticationProperties = properties
	}
	return &providers.AuthorizationDetailType{
		ID:              doc.ID,
		Type:            doc.Type,
		Name:            doc.Name,
		Description:     doc.Description,
		Version:         1,
		Schema:          schema,
		ConsentTemplate: doc.ConsentTemplate,
		Processor:       processor,
		Reuse:           doc.Reuse != nil && *doc.Reuse,
	}, nil
}

// writeYAMLNodeAsJSON writes a YAML value as JSON, keeping the order of the members of each mapping.
func writeYAMLNodeAsJSON(buf *bytes.Buffer, node *yaml.Node) error {
	switch node.Kind {
	case yaml.AliasNode:
		return writeYAMLNodeAsJSON(buf, node.Alias)
	case yaml.MappingNode:
		buf.WriteByte('{')
		for i := 0; i+1 < len(node.Content); i += 2 {
			if i > 0 {
				buf.WriteByte(',')
			}
			key, err := json.Marshal(node.Content[i].Value)
			if err != nil {
				return err
			}
			buf.Write(key)
			buf.WriteByte(':')
			if err := writeYAMLNodeAsJSON(buf, node.Content[i+1]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	case yaml.SequenceNode:
		buf.WriteByte('[')
		for i, item := range node.Content {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeYAMLNodeAsJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	default:
		var value interface{}
		if err := node.Decode(&value); err != nil {
			return err
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		buf.Write(encoded)
	}
	return nil
}

// loadDeclarativeResources loads declarative authorization details types from files.
func loadDeclarativeResources(fileStore *typeFileBasedStore, dbStore typeStoreInterface) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeAuthorizationDetailType,
		DirectoryName: "authorization_detail_types",
		Parser:        parseToTypeWrapper,
		Validator: func(data interface{}) error {
			return validateTypeWrapper(data, fileStore, dbStore)
		},
		IDExtractor: func(data interface{}) string {
			return data.(*providers.AuthorizationDetailType).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load authorization details type resources: %w", err)
	}
	return nil
}

// parseToTypeWrapper parses a declarative YAML document into a type.
func parseToTypeWrapper(data []byte) (interface{}, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse authorization details type YAML: %w", err)
	}
	t, err := ParseAuthorizationDetailTypeNode(&node)
	if err != nil {
		return nil, err
	}
	if svcErr := setProcessorAuthentication(t, nil); svcErr != nil {
		return nil, fmt.Errorf("invalid processor authentication for authorization details type '%s': %s",
			t.Type, svcErr.ErrorDescription.String())
	}
	return t, nil
}

// validateTypeWrapper validates a declarative type and rejects duplicate ids or type values.
func validateTypeWrapper(data interface{}, fileStore *typeFileBasedStore, dbStore typeStoreInterface) error {
	t, ok := data.(*providers.AuthorizationDetailType)
	if !ok {
		return errors.New("invalid type: expected *providers.AuthorizationDetailType")
	}
	if t.ID == "" {
		return errors.New("authorization details type ID is required")
	}
	if svcErr := validateTypeDefinition(*t); svcErr != nil {
		return fmt.Errorf("invalid authorization details type '%s': %s", t.ID, svcErr.ErrorDescription.String())
	}

	ctx := context.Background()
	if err := checkDuplicateType(ctx, t, fileStore, "declarative resources"); err != nil {
		return err
	}
	if dbStore != nil {
		return checkDuplicateType(ctx, t, dbStore, "the database store")
	}
	return nil
}

// checkDuplicateType rejects a type whose id or type value the given store already holds.
func checkDuplicateType(ctx context.Context, t *providers.AuthorizationDetailType, store typeStoreInterface,
	storeName string) error {
	if _, err := store.GetTypeByID(ctx, t.ID); err == nil {
		return fmt.Errorf("duplicate authorization details type ID '%s': the type already exists in %s",
			t.ID, storeName)
	} else if !errors.Is(err, errTypeNotFound) {
		return fmt.Errorf("failed to check for duplicate authorization details type ID '%s': %w", t.ID, err)
	}
	if _, err := store.GetTypeByValue(ctx, t.Type); err == nil {
		return fmt.Errorf("duplicate authorization details type '%s': the type value is already used in %s",
			t.Type, storeName)
	} else if !errors.Is(err, errTypeNotFound) {
		return fmt.Errorf("failed to check for duplicate authorization details type '%s': %w", t.Type, err)
	}
	return nil
}
