// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package middleware

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// DeploymentIDMiddleware puts the deployment id this request acts for on its context, so that the
// stores below read it from one place instead of each reaching for the configuration themselves.
//
// It runs for every request, so a store can rely on the context carrying an id and treat a context
// without one as what it is: a background operation rather than a request.
func DeploymentIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := deploymentIDForRequest(r); id != "" {
			r = r.WithContext(deployment.WithID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

// deploymentIDForRequest returns the deployment id a request acts for.
//
// This server holds one deployment, so every request acts for the configured identifier. It is a
// function of the request rather than a constant because that is the single point a build serving
// more than one deployment changes: it derives the id per request instead, and everything
// downstream is unaffected, because everything downstream already reads the id from the context.
var deploymentIDForRequest = func(_ *http.Request) string {
	if !config.IsServerRuntimeInitialized() {
		return ""
	}
	return config.GetServerRuntime().Config.Server.Identifier
}
