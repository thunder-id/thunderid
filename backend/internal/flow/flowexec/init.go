// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowexec

import (
	"net/http"

	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/systemflow"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize creates and configures the flow execution service components.
func Initialize(
	mux *http.ServeMux,
	flowProvider providers.FlowProvider,
	actorProvider providers.ActorProvider,
	executorRegistry executor.ExecutorRegistryInterface,
	interceptorRegistry interceptor.InterceptorRegistryInterface,
	observabilitySvc providers.ObservabilityProvider,
	cryptoSvc providers.RuntimeCryptoProvider,
	attestationVerifier providers.AttestationProvider,
	graphBuilder graphbuilder.GraphBuilderInterface,
	jwtService jwt.JWTServiceInterface,
	storeProvider providers.RuntimeStoreProvider,
	transactioner providers.Transactioner,
	serverConfigSvc serverConfigProvider,
	ssoTransport session.HandleTransportInterface,
	cfg flowconfig.Config,
) (FlowExecServiceInterface, systemflow.Runner, error) {
	flowStore := newFlowStore(storeProvider)
	interceptorRunner := newInterceptorRunner(interceptorRegistry)
	flowEngine := newFlowEngine(executorRegistry, interceptorRunner, observabilitySvc,
		flowProvider, graphBuilder)
	flowExecService := newFlowExecService(flowProvider, flowStore, flowEngine,
		actorProvider, observabilitySvc, transactioner, cryptoSvc, attestationVerifier,
		graphBuilder, jwtService, serverConfigSvc, cfg)

	// Bound the SSO handle's lifetime to the session's configured absolute timeout (same fallback as
	// the session executor's timeouts).
	sessionTimeouts := session.NewTimeouts(cfg.Session.IdleTimeoutSeconds, cfg.Session.AbsoluteTimeoutSeconds,
		cfg.Session.ActivityRefreshIntervalSeconds)
	handler := newFlowExecutionHandler(flowExecService, ssoTransport, sessionTimeouts.Absolute)
	registerRoutes(mux, handler)

	// The system flows run on the same engine as the flows the API starts, so a service that revokes
	// before it writes runs the very steps an administration flow would.
	systemFlows, err := newSystemFlowRunner(flowEngine, graphBuilder, executorRegistry)
	if err != nil {
		return nil, nil, err
	}
	return flowExecService, systemFlows, nil
}

func registerRoutes(mux *http.ServeMux, handler *flowExecutionHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /flow/execute",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleFlowExecutionRequest)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flow/execute",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
