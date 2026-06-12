package scim

const (
	loggerComponentName              = "SCIMhandler"
	scimServiceProviderConfigCreated = "2025-01-01T00:00:00Z"
	// SCIMBasePath is the base path for all SCIM v2 endpoints.
	SCIMBasePath = "/scim/v2"

	// SCIM core schema URNs.
	SCIMCoreUserSchemaURN              = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMErrorSchemaURN                 = "urn:ietf:params:scim:api:messages:2.0:Error"
	SCIMListResponseSchemaURN          = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SCIMServiceProviderConfigSchemaURN = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SCIMResourceTypeSchemaURN          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SCIMSchemaSchemaURN                = "urn:ietf:params:scim:schemas:core:2.0:Schema"

	// ThunderID custom URN parts.
	ThunderIDURNPrefix = "urn:thunderid:params:scim:schemas:"
	ThunderIDURNSuffix = ":2.0:User"
)
