// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package bff implements the backend for frontend that signs the Console in on the server, keeps
// its tokens in encrypted HttpOnly cookies, and proxies its API calls. See docs-internals/bff/spec.md.
package bff

import (
	"fmt"
	"net/http"
	"time"

	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize registers the Console BFF routes. The BFF sends its OAuth and API calls through
// inProcess, which the composition root points at the server's handler chain once that is built.
func Initialize(
	mux *http.ServeMux,
	inProcess *syshttp.InProcessTransport,
	cryptoSvc providers.RuntimeCryptoProvider,
	runtimeStore providers.RuntimeStoreProvider,
	clients providers.ActorProvider,
	verifier tokenVerifier,
) error {
	h, err := newHandler(consoleConfigFromServerRuntime(), inProcess, cryptoSvc, runtimeStore,
		newEndedSessions(runtimeStore), clients, verifier)
	if err != nil {
		return err
	}
	registerRoutes(mux, h)
	return nil
}

func newHandler(cfg Config, transport http.RoundTripper, cryptoSvc providers.RuntimeCryptoProvider,
	runtimeStore providers.RuntimeStoreProvider, ended *endedSessions,
	clients providers.ActorProvider, verifier tokenVerifier) (*handler, error) {
	httpClient := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "BFF"))
	seal := sealer{crypto: cryptoSvc}
	sessionCookie := newCookieSpec(cfg.CookieName+"_session", "/", cfg.SecureCookies)

	var sessions sessionStore
	switch cfg.SessionMode {
	case SessionModeStateful:
		if cfg.IdleTimeout <= 0 {
			return nil, fmt.Errorf("the session idle timeout must be positive, got %d", cfg.IdleTimeout)
		}
		stateful := &statefulSessions{
			store:       runtimeStore,
			sealer:      seal,
			cookie:      sessionCookie,
			idleTimeout: cfg.IdleTimeout,
			now:         time.Now,
			logger:      logger,
			refreshWait: 5 * time.Second,
			refreshPoll: 100 * time.Millisecond,
		}
		ended.forget = stateful.forget
		sessions = stateful
	case SessionModeStateless:
		sessions = &statelessSessions{
			sealer:        seal,
			sessionCookie: sessionCookie,
			refreshCookie: newCookieSpec(cfg.CookieName+"_refresh", cfg.BasePath+"/refresh", cfg.SecureCookies),
			now:           time.Now,
		}
	default:
		return nil, fmt.Errorf("unknown session mode %q, expected %q or %q", cfg.SessionMode,
			SessionModeStateful, SessionModeStateless)
	}

	return &handler{
		cfg:               cfg,
		oidc:              &oidcClient{httpClient: httpClient, issuerURL: cfg.PublicURL, clientID: cfg.ClientID},
		upstream:          httpClient,
		sealer:            seal,
		sessions:          sessions,
		ended:             ended,
		verifier:          verifier,
		clients:           clients,
		logger:            logger,
		transactionCookie: newCookieSpec(cfg.CookieName+"_txn", cfg.BasePath+"/callback", cfg.SecureCookies),
		now:               time.Now,
	}, nil
}

func registerRoutes(mux *http.ServeMux, h *handler) {
	base := h.cfg.BasePath
	mux.HandleFunc("GET "+base+"/signin", h.handleSignIn)
	mux.HandleFunc("GET "+base+"/callback", h.handleCallback)
	mux.HandleFunc("GET "+base+"/session", h.requireSameOrigin(h.handleSession))
	mux.HandleFunc("GET "+base+"/checksession", h.requireSameOrigin(h.handleCheckSession))
	mux.HandleFunc("POST "+base+"/refresh", h.requireSameOrigin(h.handleRefresh))
	mux.HandleFunc("POST "+base+"/signout", h.requireSameOrigin(h.handleSignOut))
	mux.HandleFunc(base+"/proxy/{path...}", h.requireSameOrigin(h.handleProxy))
	mux.HandleFunc("POST "+base+"/backchannel-logout", h.handleBackchannelLogout)
}
