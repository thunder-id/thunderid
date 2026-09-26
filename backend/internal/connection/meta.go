// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// connectionMetaResponse is the payload for GET /connections/meta: the server-driven
// description of a vendor's configurable options. Only authentication is described today; the
// rest of a connection's form is still statically known to the console.
type connectionMetaResponse struct {
	Vendor         string                    `json:"vendor"`
	Authentication connectionAuthMetaSection `json:"authentication"`
}

// connectionAuthMetaSection lists the outbound authentication methods a vendor supports, in the
// order a console should offer them. The methods are served verbatim from the outboundauth
// package, so a new method reaches the console without a change here.
//
// Each method's fields are an ordered array rather than a keyed object: encoding/json sorts map
// keys, which would put a password above the username it belongs under, with no way for the
// server to express render order.
type connectionAuthMetaSection struct {
	Methods []outboundauth.Method `json:"methods"`
}

// authTypesForVendor returns the outbound authentication methods a connection vendor supports,
// and whether the vendor is registered at all. A registered vendor that has no configurable
// choice, such as Twilio, reports an empty set.
func authTypesForVendor(vendor string) ([]outboundauth.Type, bool) {
	for _, candidate := range emailBackedVendors {
		if candidate.name == vendor {
			return candidate.authTypes, true
		}
	}
	for _, candidate := range smsBackedVendors {
		if candidate.name == vendor {
			return candidate.authTypes, true
		}
	}
	for _, candidate := range idpBackedVendors {
		if candidate.name == vendor {
			return nil, true
		}
	}
	return nil, false
}

// connectionMeta returns the metadata describing a connection vendor's configurable options.
func connectionMeta(vendor string) (*connectionMetaResponse, *tidcommon.ServiceError) {
	authTypes, ok := authTypesForVendor(vendor)
	if !ok {
		return nil, &ErrorInvalidConnectionVendor
	}

	return &connectionMetaResponse{
		Vendor: vendor,
		Authentication: connectionAuthMetaSection{
			Methods: outboundauth.GetMethods(authTypes),
		},
	}, nil
}
