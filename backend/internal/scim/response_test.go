// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type marshalErrorStruct struct{}

// MarshalJSON handles marshal json.
func (marshalErrorStruct) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("forced encode error")
}

// TestWriteSCIMSuccessResponse_EncodeError tests Write SCIM Success Response for Encode Error.
func TestWriteSCIMSuccessResponse_EncodeError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeSCIMSuccessResponse(context.Background(), rr, http.StatusOK, marshalErrorStruct{})
	require.Equal(t, http.StatusInternalServerError, rr.Code)
}

// TestWriteSCIMErrorResponse_EmptySchemas tests Write SCIM Error Response for Empty Schemas.
func TestWriteSCIMErrorResponse_EmptySchemas(t *testing.T) {
	rr := httptest.NewRecorder()
	writeSCIMErrorResponse(context.Background(), rr, http.StatusBadRequest, SCIMErrorResponse{
		Status: "400",
	})
	require.Equal(t, http.StatusBadRequest, rr.Code)
	var errResp SCIMErrorResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	require.Equal(t, []string{SCIMErrorSchemaURN}, errResp.Schemas)
}

// TestMapSCIMError_UncoveredCodes tests Map SCIM Error for Uncovered Codes.
func TestMapSCIMError_UncoveredCodes(t *testing.T) {
	status1, scimType1 := mapSCIMError(&ErrorInvalidPatchPath)
	require.Equal(t, http.StatusBadRequest, status1)
	require.Equal(t, scimErrorTypeInvalidPath, scimType1)

	status2, scimType2 := mapSCIMError(&ErrorInternalServer)
	require.Equal(t, http.StatusInternalServerError, status2)
	require.Equal(t, "", scimType2)

	status3, scimType3 := mapSCIMError(&ErrorFilterNotSupported)
	require.Equal(t, http.StatusBadRequest, status3)
	require.Equal(t, "invalidFilter", scimType3)

	status4, scimType4 := mapSCIMError(&ErrorPreconditionFailed)
	require.Equal(t, http.StatusPreconditionFailed, status4)
	require.Equal(t, "", scimType4)

	// Default case
	defaultErr := &tidcommon.ServiceError{Code: "BOGUS-CODE"}
	status5, scimType5 := mapSCIMError(defaultErr)
	require.Equal(t, http.StatusBadRequest, status5)
	require.Equal(t, scimErrorTypeInvalidValue, scimType5)

	// RFC 7644 §3.12: mutability violations must report scimType "mutability".
	status6, scimType6 := mapSCIMError(&ErrorImmutableUserType)
	require.Equal(t, http.StatusBadRequest, status6)
	require.Equal(t, scimErrorTypeMutability, scimType6)

	status7, scimType7 := mapSCIMError(&ErrorUnsupportedMemberType)
	require.Equal(t, http.StatusBadRequest, status7)
	require.Equal(t, scimErrorTypeInvalidValue, scimType7)
}

// TestMapSCIMError_CoversAllDefinedErrors guards against the class of bug where a new
// Error* var is added to error_constants.go but never wired into mapSCIMError's switch,
// silently falling through to the default 400 invalidValue branch.
// TestMapSCIMError_CoversAllDefinedErrors tests Map SCIM Error for Covers All Defined Errors.
func TestMapSCIMError_CoversAllDefinedErrors(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(thisFile)

	definedErrors := definedErrorVarNames(t, filepath.Join(dir, "error_constants.go"))
	handledErrors := handledErrorVarNamesInSwitch(t, filepath.Join(dir, "response.go"), "mapSCIMError")

	var missing []string
	for _, name := range definedErrors {
		if !handledErrors[name] {
			missing = append(missing, name)
		}
	}
	require.Empty(t, missing,
		"Error* vars defined in error_constants.go but not referenced in mapSCIMError's switch: %v", missing)
}

// definedErrorVarNames handles defined error var names.
func definedErrorVarNames(t *testing.T, file string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	require.NoError(t, err)

	var names []string
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, ident := range valueSpec.Names {
				if strings.HasPrefix(ident.Name, "Error") {
					names = append(names, ident.Name)
				}
			}
		}
	}
	return names
}

// handledErrorVarNamesInSwitch returns the set of local "ErrorXxx" identifiers referenced
// as "ErrorXxx.Code" in any case clause of the named function's top-level switch statement.
// handledErrorVarNamesInSwitch handles handled error var names in switch.
func handledErrorVarNamesInSwitch(t *testing.T, file, funcName string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, 0)
	require.NoError(t, err)

	handled := make(map[string]bool)
	for _, decl := range f.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Name.Name != funcName {
			continue
		}
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			caseClause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range caseClause.List {
				sel, ok := expr.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Code" {
					continue
				}
				if ident, ok := sel.X.(*ast.Ident); ok {
					handled[ident.Name] = true
				}
			}
			return true
		})
	}
	return handled
}

// TestParseSCIMPagination tests Parse SCIM Pagination.
func TestParseSCIMPagination(t *testing.T) {
	t.Run("ValidParams", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=5&count=20", nil)
		start, count := parseSCIMPaginationQueryParams(req)
		require.Equal(t, 5, start)
		require.Equal(t, 20, count)
	})

	t.Run("MissingParamsUseDefaults", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?startIndex=invalid", nil)
		start, count := parseSCIMPaginationQueryParams(req)
		require.Equal(t, 1, start)
		require.Equal(t, constants.DefaultPageSize, count)
	})

	t.Run("NegativeCountInterpretedAsZero", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=-10", nil)
		_, count := parseSCIMPaginationQueryParams(req)
		require.Equal(t, 0, count)
	})

	t.Run("ExplicitZeroCountPreserved", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?count=0", nil)
		_, count := parseSCIMPaginationQueryParams(req)
		require.Equal(t, 0, count)
	})

	t.Run("ClampedCount", func(t *testing.T) {
		count := 500
		start, resolved := normalizeSCIMPagination(0, &count)
		require.Equal(t, 1, start)
		require.Equal(t, 100, resolved)
	})

	t.Run("NilCountUsesDefault", func(t *testing.T) {
		start, count := normalizeSCIMPagination(0, nil)
		require.Equal(t, 1, start)
		require.Equal(t, constants.DefaultPageSize, count)
	})
}
