/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package envmgr serves the environment manager: the environments configuration is promoted
// through, the versions captured from them, and the work queued for each environment's data plane.
package envmgr

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/thunder-id/thunderid/internal/envmgr/model"
	"github.com/thunder-id/thunderid/internal/envmgr/service"
)

// Server routes HTTP requests to the application service.
type Server struct {
	svc *service.Service
}

// New builds a Server.
func New(svc *service.Service) *Server {
	return &Server{svc: svc}
}

// ProtectedHandler returns the full handler with protect applied to every route except the health
// check, which stays open so infrastructure probes do not need a token.
func (s *Server) ProtectedHandler(protect func(http.Handler) http.Handler) http.Handler {
	root := http.NewServeMux()
	root.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	root.Handle("/", protect(s.Handler()))
	return root
}

// Handler returns the HTTP handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /environments", s.createEnvironment)
	mux.HandleFunc("GET /environments", s.listEnvironments)
	mux.HandleFunc("GET /environments/{id}", s.getEnvironment)
	mux.HandleFunc("PUT /environments/{id}/edges", s.updateEnvironmentEdges)
	mux.HandleFunc("DELETE /environments/{id}", s.deleteEnvironment)

	mux.HandleFunc("POST /environments/{id}/versions", s.createVersion)
	mux.HandleFunc("GET /environments/{id}/versions", s.listVersions)
	mux.HandleFunc("GET /environments/{id}/versions/{seq}", s.getVersion)
	mux.HandleFunc("GET /environments/{id}/diff", s.diff)
	mux.HandleFunc("GET /environments/{id}/variables", s.checkVariables)
	mux.HandleFunc("POST /environments/{id}/apply", s.apply)
	mux.HandleFunc("POST /environments/{id}/revert", s.revert)

	mux.HandleFunc("GET /environments/{id}/secrets", s.listSecrets)
	mux.HandleFunc("PUT /environments/{id}/secrets/{name}", s.setSecret)
	mux.HandleFunc("POST /environments/{id}/secrets/{name}/regenerate", s.regenerateSecret)

	mux.HandleFunc("POST /apply", s.applyAll)
	mux.HandleFunc("PUT /tenants/{deploymentId}/secrets/{name}", s.captureSecret)

	mux.HandleFunc("GET /promotions/preview", s.promotePreview)
	mux.HandleFunc("POST /promotions", s.promote)
	return mux
}

// ---- environment handlers ----

type createEnvironmentRequest struct {
	Name string `json:"name"`
	Rank *int   `json:"rank,omitempty"`
	// PromotesTo are the environments this one can promote into. Omit to fall back to the next
	// environment by rank.
	PromotesTo []string     `json:"promotesTo,omitempty"`
	Target     model.Target `json:"target"`
	// ManagedByControlPlane marks this the one environment the control plane administers directly, so
	// it is where a credential created in the workspace is issued.
	ManagedByControlPlane bool `json:"managedByControlPlane,omitempty"`
}

// regenerateDataPlaneToken issues a new token for an environment's data plane and returns it once.
//
// The previous token stops working immediately, so that data plane drops until the new one is in
// place. That is what a rotation means; a credential that still worked afterwards would not have been
// rotated.
func (s *Server) regenerateDataPlaneToken(w http.ResponseWriter, r *http.Request) {
	token, err := s.svc.RegenerateDataPlaneToken(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"dataPlaneToken": token})
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var req createEnvironmentRequest
	if !decode(w, r, &req) {
		return
	}
	env, err := s.svc.CreateEnvironment(r.Context(), service.CreateEnvironmentInput{
		Name: req.Name, Rank: req.Rank, PromotesTo: req.PromotesTo, Target: req.Target,
		ManagedByControlPlane: req.ManagedByControlPlane,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, env)
}

// updateEnvironmentEdges replaces an environment's outgoing promotion edges.
func (s *Server) updateEnvironmentEdges(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PromotesTo []string `json:"promotesTo"`
	}
	if !decode(w, r, &req) {
		return
	}
	env, err := s.svc.UpdateEnvironmentEdges(r.Context(), r.PathValue("id"), req.PromotesTo)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.svc.ListEnvironmentSummaries(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	// Whether this caller may promote travels with the list, so the console can leave the action out
	// rather than offer it and have the request refused.
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"environments": summaries,
		"canPromote":   callerMayPromote(r),
	})
}

// setManagedEnvironment moves the mark for the environment the control plane administers directly.
func (s *Server) setManagedEnvironment(w http.ResponseWriter, r *http.Request) {
	env, err := s.svc.SetManagedEnvironment(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

// getDataPlaneJob returns work queued for a data plane and, once delivered, what it answered.
//
// An apply or a credential is carried out by the pod holding that data plane's connection, which is
// not always the pod that took the request. When it is not, the caller is given a job id and reads
// the answer back here, from whichever pod serves the poll.
func (s *Server) getDataPlaneJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.svc.JobStatus(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) getEnvironment(w http.ResponseWriter, r *http.Request) {
	env, err := s.svc.GetEnvironment(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, env)
}

func (s *Server) deleteEnvironment(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.DeleteEnvironment(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- version handlers ----

type createVersionRequest struct {
	Mode      string            `json:"mode,omitempty"` // "capture" (default) or "upload"
	Note      string            `json:"note,omitempty"`
	Resources string            `json:"resources,omitempty"`
	Variables map[string]string `json:"variables,omitempty"`
}

func (s *Server) createVersion(w http.ResponseWriter, r *http.Request) {
	envID := r.PathValue("id")
	var req createVersionRequest
	if !decode(w, r, &req) {
		return
	}
	mode := req.Mode
	if mode == "" {
		if req.Resources != "" {
			mode = "upload"
		} else {
			mode = "capture"
		}
	}
	var (
		version model.Version
		err     error
	)
	switch mode {
	case "capture":
		version, err = s.svc.CaptureVersion(r.Context(), envID, req.Note)
	case "upload":
		version, err = s.svc.UploadVersion(r.Context(), envID, req.Resources, req.Variables, req.Note)
	default:
		writeErrorStatus(w, http.StatusBadRequest, "mode must be 'capture' or 'upload'")
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, stripVersionPayload(version))
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.svc.ListVersions(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"versions": versions})
}

func (s *Server) getVersion(w http.ResponseWriter, r *http.Request) {
	seq, ok := parseSeq(w, r.PathValue("seq"))
	if !ok {
		return
	}
	version, err := s.svc.GetVersion(r.Context(), r.PathValue("id"), seq)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, version)
}

// checkVariables reports whether a version's placeholders would all resolve.
func (s *Server) checkVariables(w http.ResponseWriter, r *http.Request) {
	status, err := s.svc.CheckVariables(r.Context(), r.PathValue("id"), r.URL.Query().Get("version"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) diff(w http.ResponseWriter, r *http.Request) {
	from := defaultQuery(r, "from", "applied")
	to := defaultQuery(r, "to", "latest")
	d, err := s.svc.Diff(r.Context(), r.PathValue("id"), from, to)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// ---- apply / revert ----

type applyRequest struct {
	Version string `json:"version,omitempty"`
	DryRun  bool   `json:"dryRun,omitempty"`
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	var req applyRequest
	if !decode(w, r, &req) {
		return
	}
	result, err := s.svc.Apply(r.Context(), r.PathValue("id"), req.Version, req.DryRun)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// applyAll re-applies every environment's latest version, which is how a changed value such as a
// redirect URL reaches the data planes without editing the configuration itself.
func (s *Server) applyAll(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DryRun bool `json:"dryRun,omitempty"`
	}
	if !decode(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": s.svc.ApplyAll(r.Context(), req.DryRun),
	})
}

// captureSecret relays a secret the control plane captured to the secret provider of whichever
// environment that tenant belongs to.
//
// The control plane serves every tenant, so it cannot address a provider directly; it sends here and
// this service routes by tenant. The body is the provider's own write shape and is passed through
// untouched.
// listSecrets reports every credential an environment needs and whether its data plane holds it.
func (s *Server) listSecrets(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListSecrets(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// setSecret writes one credential to the environment's data plane. How it is held is decided by the
// configuration that uses it, not by the caller, so the body carries only the value.
func (s *Server) setSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value string `json:"value"`
	}
	if !decode(w, r, &req) {
		return
	}
	entry, err := s.svc.SetSecret(r.Context(), r.PathValue("id"), r.PathValue("name"), req.Value)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// regenerateSecret issues a new credential and returns it. This is the only response that carries a
// secret value, because a hashed credential cannot be read back afterwards.
func (s *Server) regenerateSecret(w http.ResponseWriter, r *http.Request) {
	entry, value, err := s.svc.RegenerateSecret(r.Context(), r.PathValue("id"), r.PathValue("name"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"secret": entry, "value": value})
}

func (s *Server) captureSecret(w http.ResponseWriter, r *http.Request) {
	var body map[string]interface{}
	if !decode(w, r, &body) {
		return
	}
	delivered, err := s.svc.CaptureSecretForTenant(r.Context(), r.PathValue("deploymentId"),
		r.PathValue("name"), body)
	if err != nil {
		writeError(w, err)
		return
	}
	// Zero deliveries is not an error: no environment is registered for the tenant yet, and a promote
	// creates the credential against the target when one is.
	writeJSON(w, http.StatusOK, map[string]int{"delivered": delivered})
}

type revertRequest struct {
	// ToVersion is a version number, or "previous" for the version immediately before the head.
	ToVersion string `json:"toVersion"`
	Apply     bool   `json:"apply,omitempty"`
	DryRun    bool   `json:"dryRun,omitempty"`
	Note      string `json:"note,omitempty"`
}

func (s *Server) revert(w http.ResponseWriter, r *http.Request) {
	var req revertRequest
	if !decode(w, r, &req) {
		return
	}
	if req.ToVersion == "" {
		req.ToVersion = "previous"
	}
	result, err := s.svc.Revert(r.Context(), service.RevertInput{
		EnvID: r.PathValue("id"), ToRef: req.ToVersion, Apply: req.Apply, DryRun: req.DryRun, Note: req.Note,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- promotion ----

func (s *Server) promotePreview(w http.ResponseWriter, r *http.Request) {
	fromEnv := r.URL.Query().Get("fromEnv")
	toEnv := r.URL.Query().Get("toEnv")
	if fromEnv == "" || toEnv == "" {
		writeErrorStatus(w, http.StatusBadRequest, "fromEnv and toEnv are required")
		return
	}
	d, err := s.svc.PromotePreview(r.Context(), fromEnv, toEnv, r.URL.Query().Get("version"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type promoteRequest struct {
	FromEnv string `json:"fromEnv"`
	ToEnv   string `json:"toEnv"`
	Version string `json:"version,omitempty"`
	// Selection is a pointer so that an omitted field ("no preference, use what was remembered") is
	// distinguishable from an empty list ("hold everything back this time").
	Selection *[]string `json:"selection,omitempty"`
	Apply     bool      `json:"apply,omitempty"`
	DryRun    bool      `json:"dryRun,omitempty"`
	Note      string    `json:"note,omitempty"`
}

func (s *Server) promote(w http.ResponseWriter, r *http.Request) {
	var req promoteRequest
	if !decode(w, r, &req) {
		return
	}
	if req.FromEnv == "" || req.ToEnv == "" {
		writeErrorStatus(w, http.StatusBadRequest, "fromEnv and toEnv are required")
		return
	}
	var selection []string
	if req.Selection != nil {
		selection = *req.Selection
	}
	result, err := s.svc.Promote(r.Context(), service.PromoteInput{
		FromEnvID: req.FromEnv, ToEnvID: req.ToEnv, VersionRef: req.Version,
		Selection: selection, SelectionProvided: req.Selection != nil,
		Apply: req.Apply, DryRun: req.DryRun, Note: req.Note,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---- helpers ----

func decode(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true // allow empty bodies; zero-value request
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErrorStatus(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

func parseSeq(w http.ResponseWriter, raw string) (int, bool) {
	seq, err := strconv.Atoi(raw)
	if err != nil || seq <= 0 {
		writeErrorStatus(w, http.StatusBadRequest, "invalid version sequence")
		return 0, false
	}
	return seq, true
}

func defaultQuery(r *http.Request, key, fallback string) string {
	if v := r.URL.Query().Get(key); v != "" {
		return v
	}
	return fallback
}

func stripVersionPayload(v model.Version) model.Version {
	v.Resources = ""
	v.Variables = nil
	return v
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		writeErrorStatus(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrValidation),
		errors.Is(err, service.ErrNoWorkspace),
		errors.Is(err, service.ErrNoVersions),
		errors.Is(err, service.ErrNothingApplied),
		errors.Is(err, service.ErrNoPreviousVersion),
		errors.Is(err, service.ErrNoPromotionPath),
		errors.Is(err, service.ErrBadRef):
		writeErrorStatus(w, http.StatusBadRequest, err.Error())
	default:
		writeErrorStatus(w, http.StatusInternalServerError, err.Error())
	}
}

func writeErrorStatus(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
