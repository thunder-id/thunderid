package scim

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSCIMUserRequest(t *testing.T) {
	validURN := "urn:thunderid:params:scim:schemas:employee:2.0:User"

	tests := []struct {
		name         string
		body         []byte
		wantErrCode  string
		wantUserType string
		wantExtURN   string
	}{
		{
			name:        "InvalidJSON",
			body:        []byte(`not json`),
			wantErrCode: ErrorInvalidRequestBody.Code,
		},
		{
			name:        "MissingSchemas",
			body:        []byte(`{"userName":"alice"}`),
			wantErrCode: ErrorMissingSchemas.Code,
		},
		{
			name:        "EmptySchemas",
			body:        []byte(`{"schemas":[],"` + validURN + `":{}}`),
			wantErrCode: ErrorMissingSchemas.Code,
		},
		{
			name:        "DuplicateSchemas",
			body:        []byte(`{"schemas":["` + validURN + `","` + validURN + `"],"` + validURN + `":{}}`),
			wantErrCode: ErrorDuplicateSchemas.Code,
		},
		{
			name:        "MissingThunderIDURN",
			body:        []byte(`{"schemas":["urn:ietf:params:scim:schemas:core:2.0:User"]}`),
			wantErrCode: ErrorMissingCustomSchema.Code,
		},
		{
			name:         "CoreOnly_NoThunderIDURN_ParsesWithEmptyUserType",
			body:         []byte(`{"schemas":["` + SCIMCoreUserSchemaURN + `"],"userName":"alice"}`),
			wantErrCode:  "",
			wantUserType: "",
			wantExtURN:   "",
		},
		{
			name: "MultipleThunderIDURNs",
			body: []byte(`{` +
				`"schemas":["urn:thunderid:params:scim:schemas:employee:2.0:User",` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User"],` +
				`"urn:thunderid:params:scim:schemas:employee:2.0:User":{},` +
				`"urn:thunderid:params:scim:schemas:person:2.0:User":{}}`),
			wantErrCode: ErrorMultipleCustomSchemas.Code,
		},
		{
			name: "MalformedCustomSchemaURN_WrongSuffix",
			body: []byte(
				`{"schemas":["urn:thunderid:params:scim:schemas:employee:2.0:Group"],` +
					`"urn:thunderid:params:scim:schemas:employee:2.0:Group":{}}`),
			wantErrCode: ErrorInvalidCustomSchemaURN.Code,
		},
		{
			name: "MalformedCustomSchemaURN_EmptyUserType",
			body: []byte(
				`{"schemas":["urn:thunderid:params:scim:schemas::2.0:User"],` +
					`"urn:thunderid:params:scim:schemas::2.0:User":{}}`),
			wantErrCode: ErrorInvalidCustomSchemaURN.Code,
		},
		{
			name:         "OmittedExtensionObject_DefaultsToEmpty",
			body:         []byte(`{"schemas":["` + SCIMCoreUserSchemaURN + `","` + validURN + `"],"userName":"alice"}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:        "InvalidExtensionObjectJSON",
			body:        []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":"not-an-object"}`),
			wantErrCode: ErrorMissingCustomSchemaObject.Code,
		},
		{
			name: "ValidPayload",
			body: []byte(`{
				"schemas":["` + SCIMCoreUserSchemaURN + `","` + validURN + `"],
				"` + validURN + `":{"department":"engineering"},
				"userName":"alice"
			}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:         "ValidPayload_ExtensionOnly_NoCoreAttrs_CoreSchemaOmitted",
			body:         []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":{"given_name":"alice"}}`),
			wantErrCode:  "",
			wantUserType: "employee",
			wantExtURN:   validURN,
		},
		{
			name:        "CoreAttrsPresent_CoreUserSchemaMissing",
			body:        []byte(`{"schemas":["` + validURN + `"],"` + validURN + `":{}, "userName":"alice"}`),
			wantErrCode: ErrorMissingCoreUserSchema.Code,
		},
		{
			name: "UndeclaredThunderIDExtensionKey_NoThunderIDURNInSchemas",
			body: []byte(`{"schemas":["` + SCIMCoreUserSchemaURN + `"],` +
				`"` + validURN + `":{"department":"engineering"},"userName":"alice"}`),
			wantErrCode: ErrorUndeclaredCustomSchemaObject.Code,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			payload, svcErr := ValidateSCIMUserRequest(tc.body)
			if tc.wantErrCode != "" {
				require.NotNil(t, svcErr, "expected a ServiceError")
				require.Equal(t, tc.wantErrCode, svcErr.Code)
				require.Nil(t, payload)
				return
			}
			require.Nil(t, svcErr)
			require.NotNil(t, payload)
			require.Equal(t, tc.wantUserType, payload.UserTypeName)
			require.Equal(t, tc.wantExtURN, payload.ExtensionURN)
		})
	}
}
