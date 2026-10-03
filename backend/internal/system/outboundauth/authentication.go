// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package outboundauth

// Authentication is the wire form of an outbound authentication configuration: a discriminator
// plus a property bag whose keys come from the registered method. It is the one shape every
// resource that dials out with credentials carries, on the REST API, in a declarative document
// and in export output alike, so the three always read the same. Keeping the bag generic is
// what lets a new method be added without a schema change.
//
// Type is a plain string rather than a Type so an unrecognized value survives decoding and is
// rejected by Validate with a descriptive message. On a response the secret values are masked.
type Authentication struct {
	Type       string            `yaml:"type"                 json:"type"`
	Properties map[string]string `yaml:"properties,omitempty" json:"properties,omitempty"`
}

// Config maps the wire object onto a configuration. A nil object, such as an omitted block,
// means "authenticates nothing".
func (a *Authentication) Config() Config {
	if a == nil {
		return Config{Type: TypeNone}
	}

	// An unrecognized type is passed through rather than normalized, so Validate rejects it
	// with a descriptive error instead of the configuration silently becoming "none".
	authType := Type(a.Type)
	if parsed, ok := ParseType(a.Type); ok {
		authType = parsed
	}

	properties := make(map[string]string, len(a.Properties))
	for name, value := range a.Properties {
		properties[name] = value
	}

	return Config{Type: authType, Properties: properties}
}

// AuthenticationFromValues rebuilds the wire object from an already-resolved property-key to
// value map. Callers pass the masked map their read path produced, so a secret cannot be read
// back in plain text through this function.
func AuthenticationFromValues(values map[string]string) Authentication {
	cfg := FromValues(values)

	authType := cfg.Type
	if authType == "" {
		authType = TypeNone
	}

	return Authentication{
		Type:       string(authType),
		Properties: cfg.Properties,
	}
}
