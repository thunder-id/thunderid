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

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	resourceTypeUser = "user"
	paramTypeUser    = "User"
)

// userExporter implements declarativeresource.ResourceExporter for users.
type userExporter struct {
	service       UserServiceInterface
	entityService entity.EntityServiceInterface
}

// newUserExporter creates a new user exporter.
func newUserExporter(service UserServiceInterface, entityService entity.EntityServiceInterface) *userExporter {
	return &userExporter{service: service, entityService: entityService}
}

// GetResourceType returns the resource type for users.
func (e *userExporter) GetResourceType() string {
	return resourceTypeUser
}

// GetParameterizerType returns the parameterizer type for users.
func (e *userExporter) GetParameterizerType() string {
	return paramTypeUser
}

// GetAllResourceIDs retrieves all user IDs from the database store.
// In composite mode, this excludes declarative (YAML-based) users.
func (e *userExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	ids := []string{}

	for {
		users, err := e.service.GetUserList(ctx, limit, offset, nil, false)
		if err != nil {
			return nil, err
		}

		for _, user := range users.Users {
			isDeclarative, declErr := e.entityService.IsEntityDeclarative(ctx, user.ID)
			if declErr != nil {
				if errors.Is(declErr, entity.ErrEntityNotFound) {
					ids = append(ids, user.ID)
					continue
				}
				return nil, &tidcommon.InternalServerError
			}
			if !isDeclarative {
				ids = append(ids, user.ID)
			}
		}

		offset += len(users.Users)

		// Continue fetching while we get results; stop only on empty page
		if len(users.Users) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves a user by its ID.
func (e *userExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError) {
	user, err := e.service.GetUser(ctx, id, false)
	if err != nil {
		return nil, "", err
	}

	// Extract username from attributes for identification
	var username string
	var attrs map[string]interface{}
	if len(user.Attributes) > 0 {
		if jsonErr := json.Unmarshal(user.Attributes, &attrs); jsonErr == nil {
			if un, ok := attrs["username"].(string); ok {
				username = un
			}
		}
	}

	// Convert User.Attributes (json.RawMessage) to map for export
	var attributesMap map[string]interface{}
	if len(user.Attributes) > 0 {
		if jsonErr := json.Unmarshal(user.Attributes, &attributesMap); jsonErr != nil {
			attributesMap = make(map[string]interface{})
		}
	} else {
		attributesMap = make(map[string]interface{})
	}

	// Create export structure with credentials as placeholders
	// The parameterizer will replace actual credential values with template variables
	exportUser := &userDeclarativeResource{
		ID:          user.ID,
		Type:        user.Type,
		OUID:        user.OUID,
		Attributes:  attributesMap,
		Credentials: make(map[string]interface{}), // Empty credentials - will be filled with placeholders
	}

	return exportUser, username, nil
}

// ValidateResource validates a user resource.
func (e *userExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	user, ok := resource.(*userDeclarativeResource)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeUser, id)
	}

	// Extract username for validation
	var username string
	if un, ok := user.Attributes["username"].(string); ok {
		username = un
	}

	if username == "" {
		logger.Warn(ctx, "USER_VALIDATION_ERROR: Missing username",
			log.MaskedString(log.LoggerKeyUserID, id))
		return "", &declarativeresource.ExportError{
			ResourceType: resourceTypeUser,
			ResourceID:   id,
			Code:         "USER_VALIDATION_ERROR",
			Error:        fmt.Sprintf("User '%s' validation failed: missing username", id),
		}
	}

	return username, nil
}

// GetResourceRules returns the parameterization rules for users.
func (e *userExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables:             []string{},
		ArrayVariables:        []string{},
		DynamicPropertyFields: []string{"Credentials"},
	}
}

// makeUserDeclarativeConfig creates the declarative loader configuration for user resources.
// This provides user-specific parser and validator callbacks to the entity service.
// When userService is non-nil, ou_handle is resolved to ou_id during parsing. ouService and
// entityTypeService are used to validate references without requiring an authenticated actor,
// since declarative resources are loaded at startup outside any request context.
func makeUserDeclarativeConfig(
	userService UserServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) entity.DeclarativeLoaderConfig {
	return entity.DeclarativeLoaderConfig{
		Directory: "users",
		Category:  providers.EntityCategoryUser,
		Parser:    makeUserParser(userService),
		Validator: makeUserValidator(ouService, entityTypeService),
	}
}

// makeUserParser creates a parser callback that converts YAML data into an Entity with credentials.
// When userService is non-nil, ou_handle is resolved to ou_id before producing the entity.
func makeUserParser(
	userService UserServiceInterface,
) func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
	return func(data []byte) (*providers.Entity, json.RawMessage, json.RawMessage, error) {
		user, creds, err := parseToUser(data)
		if err != nil {
			return nil, nil, nil, err
		}

		if userService != nil {
			if svcErr := userService.ResolveUserOUHandle(context.Background(), &user); svcErr != nil {
				return nil, nil, nil, fmt.Errorf(
					"organization unit with handle %q not found for user '%s'", user.OUHandle, user.ID)
			}
		}

		e := userToEntity(&user)
		systemCreds, err := credentialsToJSON(creds)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to marshal credentials: %w", err)
		}

		return e, nil, systemCreds, nil
	}
}

// makeUserValidator creates a validator callback for declarative user resources.
// ouService and entityTypeService validate references directly at the store/existence level,
// bypassing the authorization checks that entityTypeService.GetEntityTypeByName enforces for
// authenticated API requests, since declarative loading runs at startup with no actor in context.
func makeUserValidator(
	ouService oupkg.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
) func(e *providers.Entity, svc entity.EntityServiceInterface) error {
	return func(e *providers.Entity, svc entity.EntityServiceInterface) error {
		if e.ID == "" {
			return fmt.Errorf("user ID is required")
		}
		if e.Type == "" {
			return fmt.Errorf("user type is required")
		}
		if e.OUID == "" {
			return fmt.Errorf("ouId or ouHandle is required for user '%s'", e.ID)
		}
		if len(e.Attributes) == 0 {
			return fmt.Errorf("user attributes are required")
		}

		if ouService != nil {
			exists, svcErr := ouService.IsOrganizationUnitExists(context.Background(), e.OUID)
			if svcErr != nil {
				return fmt.Errorf("failed to verify organization unit %q for user '%s': %s",
					e.OUID, e.ID, svcErr.Error.DefaultValue)
			}
			if !exists {
				return fmt.Errorf("organization unit %q not found for user '%s'", e.OUID, e.ID)
			}
		}

		if entityTypeService != nil {
			exists, svcErr := entityTypeService.IsEntityTypeExists(
				context.Background(), entitytype.TypeCategoryUser, e.Type)
			if svcErr != nil {
				return fmt.Errorf("failed to verify user type %q for user '%s': %s",
					e.Type, e.ID, svcErr.Error.DefaultValue)
			}
			if !exists {
				return fmt.Errorf("user type %q not found for user '%s'", e.Type, e.ID)
			}
		}

		var attrs map[string]interface{}
		if err := json.Unmarshal(e.Attributes, &attrs); err != nil {
			return fmt.Errorf("failed to parse user attributes: %w", err)
		}

		un, ok := attrs["username"].(string)
		if !ok || un == "" {
			return fmt.Errorf("username is required in user attributes")
		}

		// Check for duplicates in the store (covers both DB and already-loaded file resources)
		_, err := svc.GetEntity(context.Background(), e.ID)
		if err == nil {
			return fmt.Errorf("duplicate user ID '%s': user already exists", e.ID)
		}
		if !errors.Is(err, entity.ErrEntityNotFound) {
			return fmt.Errorf("checking user existence for '%s': %w", e.ID, err)
		}

		return nil
	}
}

type userDeclarativeResource struct {
	ID          string                 `yaml:"id"`
	Type        string                 `yaml:"type"`
	OUID        string                 `yaml:"ouId,omitempty"`
	OUHandle    string                 `yaml:"ouHandle,omitempty"`
	Attributes  map[string]interface{} `yaml:"attributes"`
	Credentials map[string]interface{} `yaml:"credentials,omitempty"` // Flexible format for YAML
}

// parseToUser parses YAML data into a User and its Credentials. The ou_handle from YAML is
// populated onto User.OUHandle so callers can resolve it to an ou_id via the user service.
func parseToUser(data []byte) (User, Credentials, error) {
	var userRes userDeclarativeResource
	if err := yaml.Unmarshal(data, &userRes); err != nil {
		return User{}, nil, err
	}

	// Convert attributes map to JSON
	attributesJSON, err := json.Marshal(userRes.Attributes)
	if err != nil {
		return User{}, nil, fmt.Errorf("failed to marshal attributes: %w", err)
	}

	user := User{
		ID:         userRes.ID,
		Type:       userRes.Type,
		OUID:       userRes.OUID,
		OUHandle:   userRes.OUHandle,
		Attributes: json.RawMessage(attributesJSON),
	}

	// Parse and hash credentials
	credentials, err := parseCredentials(userRes.Credentials)
	if err != nil {
		return User{}, nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return user, credentials, nil
}

// parseCredentials parses credentials from YAML and hashes plain text values.
// Supports two formats:
// 1. Simple format: credentials: { password: "plaintext" }
// 2. Full format: credentials: { password: [{ storageType: "hash", value: "hashed", ... }] }
func parseCredentials(credentialsMap map[string]interface{}) (Credentials, error) {
	if len(credentialsMap) == 0 {
		return make(Credentials), nil
	}

	credentials := make(Credentials)
	hashCfg, err := buildHashCfgForUser()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hash service: %w", err)
	}
	hashService, err := cryptolib.Initialize(hashCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize hash service: %w", err)
	}

	for credType, credValue := range credentialsMap {
		credentialType := CredentialType(credType)

		switch v := credValue.(type) {
		case string:
			// Simple format: plain text password that needs hashing
			if v == "" {
				continue
			}

			if credentialType.IsSystemManaged() {
				credentials[credentialType] = []Credential{{Value: v}}
				continue
			}

			hashedCred, err := hashService.Generate([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("failed to hash credential %s: %w", credType, err)
			}

			credential := Credential{
				StorageType: "hash",
				StorageAlgo: hashedCred.Algorithm,
				StorageAlgoParams: cryptolib.CredParameters{
					Iterations:  hashedCred.Parameters.Iterations,
					Memory:      hashedCred.Parameters.Memory,
					Parallelism: hashedCred.Parameters.Parallelism,
					KeySize:     hashedCred.Parameters.KeySize,
					Salt:        hashedCred.Parameters.Salt,
				},
				Value: hashedCred.Hash,
			}

			credentials[credentialType] = []Credential{credential}

		case []interface{}:
			// Full format: array of credential objects
			var credList []Credential
			for _, item := range v {
				credMap, ok := item.(map[string]interface{})
				if !ok {
					// Try map[interface{}]interface{} (YAML unmarshaling)
					if credMapAny, ok := item.(map[interface{}]interface{}); ok {
						credMap = make(map[string]interface{})
						for k, val := range credMapAny {
							if keyStr, ok := k.(string); ok {
								credMap[keyStr] = val
							}
						}
					} else {
						return nil, fmt.Errorf("invalid credential format for %s", credType)
					}
				}

				cred, err := parseCredentialObject(credMap, hashService, credentialType)
				if err != nil {
					return nil, fmt.Errorf("failed to parse credential %s: %w", credType, err)
				}
				credList = append(credList, cred)
			}
			credentials[credentialType] = credList

		default:
			return nil, fmt.Errorf("unsupported credential format for %s", credType)
		}
	}

	return credentials, nil
}

// parseCredentialObject parses a single credential object.
// If the value is plain text and no hash info is provided, it will hash it.
func parseCredentialObject(
	credMap map[string]interface{},
	hashService cryptolib.HashServiceInterface,
	credentialType CredentialType,
) (Credential, error) {
	value, hasValue := credMap["value"].(string)
	if !hasValue || value == "" {
		return Credential{}, fmt.Errorf("credential value is required")
	}

	storageType, _ := credMap["storageType"].(string)
	storageAlgo, _ := credMap["storageAlgo"].(string)
	systemManaged, _ := credMap["systemManaged"].(bool)

	if credentialType.IsSystemManaged() || systemManaged || storageType == "system" {
		if storageType == "" {
			storageType = "system"
		}
		return Credential{
			StorageType: storageType,
			StorageAlgo: cryptolib.CredAlgorithm(storageAlgo),
			Value:       value,
		}, nil
	}

	// If storage type is not specified or is not "hash", treat as plain text and hash it
	if storageType == "" || storageType != "hash" {
		hashedCred, err := hashService.Generate([]byte(value))
		if err != nil {
			return Credential{}, fmt.Errorf("failed to hash credential: %w", err)
		}

		return Credential{
			StorageType: "hash",
			StorageAlgo: hashedCred.Algorithm,
			StorageAlgoParams: cryptolib.CredParameters{
				Iterations:  hashedCred.Parameters.Iterations,
				Memory:      hashedCred.Parameters.Memory,
				Parallelism: hashedCred.Parameters.Parallelism,
				KeySize:     hashedCred.Parameters.KeySize,
				Salt:        hashedCred.Parameters.Salt,
			},
			Value: hashedCred.Hash,
		}, nil
	}

	// Parse pre-hashed credential
	paramsMap, _ := credMap["storageAlgoParams"].(map[string]interface{})
	if paramsMap == nil {
		// Try map[interface{}]interface{} format
		if paramsMapAny, ok := credMap["storageAlgoParams"].(map[interface{}]interface{}); ok {
			paramsMap = make(map[string]interface{})
			for k, v := range paramsMapAny {
				if keyStr, ok := k.(string); ok {
					paramsMap[keyStr] = v
				}
			}
		}
	}

	iterations, _ := paramsMap["iterations"].(int)
	keySize, _ := paramsMap["keySize"].(int)
	salt, _ := paramsMap["salt"].(string)

	return Credential{
		StorageType: storageType,
		StorageAlgo: cryptolib.CredAlgorithm(storageAlgo),
		StorageAlgoParams: cryptolib.CredParameters{
			Iterations: iterations,
			KeySize:    keySize,
			Salt:       salt,
		},
		Value: value,
	}, nil
}

// buildHashCfgForUser constructs a cryptolib.HashConfig from the server's password hashing config.
func buildHashCfgForUser() (cryptolib.HashConfig, error) {
	cfg := config.GetServerRuntime().Config.Crypto.PasswordHashing
	alg := cryptolib.CredAlgorithm(strings.ToUpper(cfg.Algorithm))
	switch alg {
	case "", cryptolib.SHA256:
		return cryptolib.HashConfig{Algorithm: cryptolib.SHA256, SaltSize: cfg.SHA256.SaltSize}, nil
	case cryptolib.PBKDF2:
		return cryptolib.HashConfig{Algorithm: alg, SaltSize: cfg.PBKDF2.SaltSize,
			Iterations: cfg.PBKDF2.Iterations, KeySize: cfg.PBKDF2.KeySize}, nil
	case cryptolib.ARGON2ID:
		return cryptolib.HashConfig{Algorithm: alg, SaltSize: cfg.Argon2ID.SaltSize,
			Iterations: cfg.Argon2ID.Iterations, Memory: cfg.Argon2ID.Memory,
			Parallelism: cfg.Argon2ID.Parallelism, KeySize: cfg.Argon2ID.KeySize}, nil
	default:
		return cryptolib.HashConfig{}, fmt.Errorf("unrecognized password hashing algorithm %q", cfg.Algorithm)
	}
}
