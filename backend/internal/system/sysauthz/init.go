// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

// InitializeDeploymentBoundedAuthorization returns a SystemAuthorizationServiceInterface under which a
// caller acts on any organization unit in the deployment and may create root organization units.
// This package exposes no HTTP routes and requires no store — it is a pure service.
func InitializeDeploymentBoundedAuthorization() (SystemAuthorizationServiceInterface, error) {
	return newDeploymentBoundedAuthzService(), nil
}

// InitializeOUBoundedAuthorization returns a SystemAuthorizationServiceInterface under which a caller
// acts only on the organization unit its token names, and root organization units come only through
// import.
// This package exposes no HTTP routes and requires no store — it is a pure service.
func InitializeOUBoundedAuthorization() (SystemAuthorizationServiceInterface, error) {
	return newOUBoundedAuthzService(), nil
}
