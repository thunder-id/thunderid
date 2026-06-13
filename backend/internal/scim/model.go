package scim

// SCIMSupportedFeature captures a simple supported/unsupported capability flag.
type SCIMSupportedFeature struct {
	Supported bool `json:"supported"`
}

// SCIMBulkConfig captures bulk operation capability flags.
type SCIMBulkConfig struct {
	Supported      bool `json:"supported"`
	MaxOperations  int  `json:"maxOperations"`
	MaxPayloadSize int  `json:"maxPayloadSize"`
}

// SCIMFilterConfig captures filter capability flags.
type SCIMFilterConfig struct {
	Supported  bool `json:"supported"`
	MaxResults int  `json:"maxResults"`
}

// SCIMAuthenticationScheme describes one supported authentication mechanism.
type SCIMAuthenticationScheme struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SCIMMeta holds SCIM resource metadata fields.
type SCIMMeta struct {
	ResourceType string `json:"resourceType,omitempty"`
	Location     string `json:"location,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	Created      string `json:"created,omitempty"`
	Version      string `json:"version,omitempty"`
}

// SCIMServiceProviderConfig is the response body for GET /scim/v2/ServiceProviderConfig.
type SCIMServiceProviderConfig struct {
	Schemas               []string                   `json:"schemas"`
	Patch                 SCIMSupportedFeature       `json:"patch"`
	Bulk                  SCIMBulkConfig             `json:"bulk"`
	Filter                SCIMFilterConfig           `json:"filter"`
	ChangePassword        SCIMSupportedFeature       `json:"changePassword"`
	Sort                  SCIMSupportedFeature       `json:"sort"`
	ETag                  SCIMSupportedFeature       `json:"etag"`
	AuthenticationSchemes []SCIMAuthenticationScheme `json:"authenticationSchemes"`
	Meta                  SCIMMeta                   `json:"meta"`
}
