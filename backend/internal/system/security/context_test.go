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

package security

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SecurityContextTestSuite struct {
	suite.Suite
}

func TestAuthContextSuite(t *testing.T) {
	suite.Run(t, new(SecurityContextTestSuite))
}

const (
	testModifiedValue = "modified"
	testUserID        = "user123"
)

func (s *SecurityContextTestSuite) TestNewSecurityContext() {
	userID := testUserID
	ouID := "ou456"
	token := "test-token-123"
	attributes := map[string]interface{}{
		"sub":   userID,
		"roles": []string{"admin", "user"},
	}

	authCtx := newSecurityContext(userID, ouID, token, nil, attributes)

	if authCtx == nil {
		s.T().Fatal("Expected non-nil SecurityContext")
	}

	// Access the context through the getter methods
	ctx := withSecurityContext(context.Background(), authCtx)

	if GetSubject(ctx) != userID {
		s.T().Errorf("Expected userID %s, got %s", userID, GetSubject(ctx))
	}

	if GetOUID(ctx) != ouID {
		s.T().Errorf("Expected ouID %s, got %s", ouID, GetOUID(ctx))
	}
}

func (s *SecurityContextTestSuite) TestWithSecurityContext_NilContext() {
	authCtx := newSecurityContext(testUserID, "ou456", "token", nil, map[string]interface{}{
		"sub": testUserID,
	})

	ctx := withSecurityContext(nil, authCtx) //nolint:staticcheck // Testing nil context handling

	if ctx == nil {
		s.T().Fatal("Expected non-nil context")
	}

	if GetSubject(ctx) != testUserID {
		s.T().Error("Expected userID to be accessible from context created with nil base")
	}
}

func (s *SecurityContextTestSuite) TestWithSecurityContext_NilAuthContext() {
	ctx := withSecurityContext(context.Background(), nil)

	if ctx == nil {
		s.T().Fatal("Expected non-nil context even with nil SecurityContext")
	}

	// Should return empty values when no auth context is set
	if GetSubject(ctx) != "" {
		s.T().Error("Expected empty userID when SecurityContext is nil")
	}
}

func (s *SecurityContextTestSuite) TestGetUserID() {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected string
	}{
		{
			name: "Valid security context",
			setup: func() context.Context {
				authCtx := newSecurityContext(testUserID, "ou456", "token", nil, nil)
				return withSecurityContext(context.Background(), authCtx)
			},
			expected: testUserID,
		},
		{
			name: "Nil context",
			setup: func() context.Context {
				return nil
			},
			expected: "",
		},
		{
			name:     "Context without security",
			setup:    context.Background,
			expected: "",
		},
		{
			name: "Context with nil security context",
			setup: func() context.Context {
				return withSecurityContext(context.Background(), nil)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			userID := GetSubject(ctx)
			if userID != tt.expected {
				t.Errorf("Expected userID %s, got %s", tt.expected, userID)
			}
		})
	}
}

func (s *SecurityContextTestSuite) TestGetOUID() {
	s.testContextGetter("ouID", "ou456", GetOUID)
}

// testContextGetter is a helper function to test context getter functions
func (s *SecurityContextTestSuite) testContextGetter(fieldName, expectedValue string,
	getter func(context.Context) string) {
	tests := []struct {
		name     string
		setup    func() context.Context
		expected string
	}{
		{
			name: "Valid security context",
			setup: func() context.Context {
				authCtx := newSecurityContext(testUserID, "ou456", "token", nil, nil)
				return withSecurityContext(context.Background(), authCtx)
			},
			expected: expectedValue,
		},
		{
			name:     "Nil context",
			setup:    func() context.Context { return nil },
			expected: "",
		},
		{
			name:     "Context without security",
			setup:    context.Background,
			expected: "",
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			result := getter(ctx)
			if result != tt.expected {
				t.Errorf("Expected %s %s, got %s", fieldName, tt.expected, result)
			}
		})
	}
}

func (s *SecurityContextTestSuite) TestGetAttribute() {
	attributes := map[string]interface{}{
		"string_attribute": "string_value",
		"int_attribute":    42,
		"bool_attribute":   true,
		"string_slice":     []string{"a", "b", "c"},
		"interface_slice":  []interface{}{"x", "y", "z"},
		"map_attribute": map[string]interface{}{
			"nested": "value",
		},
		"nil_attribute": nil,
	}

	authCtx := newSecurityContext("user", "ou", "token", nil, attributes)
	ctx := withSecurityContext(context.Background(), authCtx)

	tests := []struct {
		name     string
		key      string
		validate func(t *testing.T, value interface{})
	}{
		{
			name: "String attribute",
			key:  "string_attribute",
			validate: func(t *testing.T, value interface{}) {
				if str, ok := value.(string); !ok || str != "string_value" {
					t.Errorf("Expected string 'string_value', got %v", value)
				}
			},
		},
		{
			name: "Int attribute",
			key:  "int_attribute",
			validate: func(t *testing.T, value interface{}) {
				if num, ok := value.(int); !ok || num != 42 {
					t.Errorf("Expected int 42, got %v", value)
				}
			},
		},
		{
			name: "Bool attribute",
			key:  "bool_attribute",
			validate: func(t *testing.T, value interface{}) {
				if b, ok := value.(bool); !ok || b != true {
					t.Errorf("Expected bool true, got %v", value)
				}
			},
		},
		{
			name: "String slice (defensive copy)",
			key:  "string_slice",
			validate: func(t *testing.T, value interface{}) {
				slice, ok := value.([]string)
				if !ok {
					t.Errorf("Expected []string, got %T", value)
					return
				}
				if len(slice) != 3 || slice[0] != "a" || slice[1] != "b" || slice[2] != "c" {
					t.Errorf("Expected [a b c], got %v", slice)
					return
				}
				// Verify it's a defensive copy by modifying it
				slice[0] = testModifiedValue
				// Get the attribute again and verify it wasn't modified
				freshAttribute := GetAttribute(ctx, "string_slice")
				freshSlice := freshAttribute.([]string)
				if freshSlice[0] != "a" {
					t.Error("Defensive copy failed - original slice was modified")
				}
			},
		},
		{
			name: "Interface slice (defensive copy)",
			key:  "interface_slice",
			validate: func(t *testing.T, value interface{}) {
				slice, ok := value.([]interface{})
				if !ok {
					t.Errorf("Expected []interface{}, got %T", value)
					return
				}
				if len(slice) != 3 {
					t.Errorf("Expected length 3, got %d", len(slice))
					return
				}
				// Verify it's a defensive copy
				slice[0] = testModifiedValue
				freshAttribute := GetAttribute(ctx, "interface_slice")
				freshSlice := freshAttribute.([]interface{})
				if freshSlice[0] != "x" {
					t.Error("Defensive copy failed - original slice was modified")
				}
			},
		},
		{
			name: "Map attribute (defensive copy)",
			key:  "map_attribute",
			validate: func(t *testing.T, value interface{}) {
				m, ok := value.(map[string]interface{})
				if !ok {
					t.Errorf("Expected map[string]interface{}, got %T", value)
					return
				}
				if m["nested"] != "value" {
					t.Errorf("Expected nested value, got %v", m["nested"])
					return
				}
				// Verify it's a defensive copy
				m["modified"] = "new_value"
				freshAttribute := GetAttribute(ctx, "map_attribute")
				freshMap := freshAttribute.(map[string]interface{})
				if _, exists := freshMap["modified"]; exists {
					t.Error("Defensive copy failed - original map was modified")
				}
			},
		},
		{
			name: "Nil attribute",
			key:  "nil_attribute",
			validate: func(t *testing.T, value interface{}) {
				if value != nil {
					t.Errorf("Expected nil, got %v", value)
				}
			},
		},
		{
			name: "Non-existent attribute",
			key:  "does_not_exist",
			validate: func(t *testing.T, value interface{}) {
				if value != nil {
					t.Errorf("Expected nil for non-existent attribute, got %v", value)
				}
			},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			value := GetAttribute(ctx, tt.key)
			tt.validate(t, value)
		})
	}

	// Test with nil context
	s.T().Run("Nil context", func(t *testing.T) {
		value := GetAttribute(nil, "any_key") //nolint:staticcheck // Testing nil context handling
		if value != nil {
			t.Errorf("Expected nil for nil context, got %v", value)
		}
	})

	// Test with context without auth
	s.T().Run("Context without auth", func(t *testing.T) {
		value := GetAttribute(context.Background(), "any_key")
		if value != nil {
			t.Errorf("Expected nil for context without auth, got %v", value)
		}
	})
}

func (s *SecurityContextTestSuite) TestGetPermissions() {
	tests := []struct {
		name                string
		setup               func() context.Context
		expectedPermissions []string
	}{
		{
			name: "With permissions set",
			setup: func() context.Context {
				authCtx := newSecurityContext("user", "ou", "token",
					[]string{"users:read", "users:write"}, nil)
				return withSecurityContext(context.Background(), authCtx)
			},
			expectedPermissions: []string{"users:read", "users:write"},
		},
		{
			name: "With empty permissions",
			setup: func() context.Context {
				authCtx := newSecurityContext("user", "ou", "token", []string{}, nil)
				return withSecurityContext(context.Background(), authCtx)
			},
			expectedPermissions: []string{},
		},
		{
			name: "With nil permissions",
			setup: func() context.Context {
				authCtx := newSecurityContext("user", "ou", "token", nil, nil)
				return withSecurityContext(context.Background(), authCtx)
			},
			expectedPermissions: []string{},
		},
		{
			name: "Nil security context",
			setup: func() context.Context {
				return withSecurityContext(context.Background(), nil)
			},
			expectedPermissions: []string{},
		},
		{
			name:                "Context without security context",
			setup:               context.Background,
			expectedPermissions: []string{},
		},
		{
			name: "Nil context",
			setup: func() context.Context {
				return nil
			},
			expectedPermissions: []string{},
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			permissions := GetPermissions(ctx)
			if len(permissions) != len(tt.expectedPermissions) {
				t.Errorf("Expected %v permissions, got %v", tt.expectedPermissions, permissions)
				return
			}
			permSet := make(map[string]struct{}, len(permissions))
			for _, p := range permissions {
				permSet[p] = struct{}{}
			}
			for _, expected := range tt.expectedPermissions {
				if _, ok := permSet[expected]; !ok {
					t.Errorf("Expected permission %q not found in %v", expected, permissions)
				}
			}
		})
	}
}

func (s *SecurityContextTestSuite) TestGetSecurityContext() {
	s.T().Run("Valid security context", func(t *testing.T) {
		authCtx := newSecurityContext("user", "ou", "token", nil, nil)
		ctx := withSecurityContext(context.Background(), authCtx)

		retrievedCtx := getSecurityContext(ctx)
		if retrievedCtx == nil {
			t.Error("Expected non-nil security context")
		}

		// Verify it's the same context by checking user ID
		if GetSubject(ctx) != "user" {
			t.Error("Retrieved context doesn't match original")
		}
	})

	s.T().Run("Nil context", func(t *testing.T) {
		retrievedCtx := getSecurityContext(nil) //nolint:staticcheck // Testing nil context handling
		if retrievedCtx != nil {
			t.Error("Expected nil security context for nil context")
		}
	})

	s.T().Run("Context without security", func(t *testing.T) {
		ctx := context.Background()
		retrievedCtx := getSecurityContext(ctx)
		if retrievedCtx != nil {
			t.Error("Expected nil security context for context without auth")
		}
	})

	s.T().Run("Context with wrong type", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), securityContextKey, "not_auth_context")
		retrievedCtx := getSecurityContext(ctx)
		if retrievedCtx != nil {
			t.Error("Expected nil security context for context with wrong type")
		}
	})
}

func (s *SecurityContextTestSuite) TestWithRuntimeContext() {
	s.T().Run("Marks context as runtime", func(t *testing.T) {
		ctx := WithRuntimeContext(context.Background())
		if !IsRuntimeContext(ctx) {
			t.Error("Expected IsRuntimeContext to return true after WithRuntimeContext")
		}
	})

	s.T().Run("Non-runtime context returns false", func(t *testing.T) {
		ctx := context.Background()
		if IsRuntimeContext(ctx) {
			t.Error("Expected IsRuntimeContext to return false for plain context")
		}
	})

	s.T().Run("Nil context returns false", func(t *testing.T) {
		if IsRuntimeContext(nil) { //nolint:staticcheck // Testing nil context handling
			t.Error("Expected IsRuntimeContext to return false for nil context")
		}
	})

	s.T().Run("Nil base context uses background", func(t *testing.T) {
		ctx := WithRuntimeContext(nil) //nolint:staticcheck // Testing nil context handling
		if ctx == nil {
			t.Fatal("Expected non-nil context from WithRuntimeContext(nil)")
		}
		if !IsRuntimeContext(ctx) {
			t.Error("Expected IsRuntimeContext to return true after WithRuntimeContext(nil)")
		}
	})

	s.T().Run("Preserves existing security context values", func(t *testing.T) {
		authCtx := newSecurityContext(testUserID, "ou456", "token", nil, nil)
		base := withSecurityContext(context.Background(), authCtx)
		ctx := WithRuntimeContext(base)
		if !IsRuntimeContext(ctx) {
			t.Error("Expected IsRuntimeContext to return true")
		}
		if GetSubject(ctx) != testUserID {
			t.Errorf("Expected subject '%s', got '%s'", testUserID, GetSubject(ctx))
		}
	})
}
