// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// connectionMetaResponse is the payload for GET /connections/meta: the server-driven
// description of each vendor's configurable options. It is a list in both modes, so filtering by
// vendor narrows the result rather than changing its shape.
type connectionMetaResponse struct {
	Vendors []connectionVendorMeta `json:"vendors"`
}

// connectionVendorMeta describes one vendor's configurable options. Only authentication is
// described today; the rest of a connection's form is still statically known to the console.
type connectionVendorMeta struct {
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
			return nil, true
		}
	}
	if isIDPBackedVendorName(vendor) {
		return nil, true
	}
	if vendor == authZENPDPVendorName {
		return nil, true
	}
	return nil, false
}

// registeredVendorNames returns every registered connection vendor, in the order the vendor
// tables declare them.
func registeredVendorNames() []string {
	names := make([]string, 0, len(idpBackedVendors)+len(smsBackedVendors)+len(emailBackedVendors)+1)
	for _, vendor := range idpBackedVendors {
		names = append(names, vendor.name)
	}
	for _, vendor := range smsBackedVendors {
		names = append(names, vendor.name)
	}
	for _, vendor := range emailBackedVendors {
		names = append(names, vendor.name)
	}
	return append(names, authZENPDPVendorName)
}

// getConnectionMeta returns the metadata describing the configurable options of every registered
// vendor, or of the named vendor alone when vendor is not empty.
func (s *service) getConnectionMeta(vendor string) (*connectionMetaResponse, *tidcommon.ServiceError) {
	vendors := registeredVendorNames()
	if vendor != "" {
		if _, ok := authTypesForVendor(vendor); !ok {
			return nil, &ErrorInvalidConnectionVendor
		}
		vendors = []string{vendor}
	}

	resp := &connectionMetaResponse{Vendors: make([]connectionVendorMeta, 0, len(vendors))}
	for _, name := range vendors {
		authTypes, _ := authTypesForVendor(name)
		resp.Vendors = append(resp.Vendors, connectionVendorMeta{
			Vendor:         name,
			Authentication: connectionAuthMetaSection{Methods: outboundauth.GetMethods(authTypes)},
		})
	}
	return resp, nil
}
