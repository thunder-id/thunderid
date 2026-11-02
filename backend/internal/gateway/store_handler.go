// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// maxStoreBody bounds a value written through to a gateway's store, which holds one value per call.
const maxStoreBody = 64 << 10

// storeHandler serves a gateway's variables and secrets through this plane.
type storeHandler struct {
	service StoreServiceInterface
}

func newStoreHandler(service StoreServiceInterface) *storeHandler {
	return &storeHandler{service: service}
}

func listHandler[T any](list func(ctx context.Context, gatewayID string,
	query StoreListQuery) (*StoreAnswer[T], *tidcommon.ServiceError)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		answer, svcErr := list(r.Context(), r.PathValue("id"), StoreListQuery{
			Limit: params.Get("limit"), Offset: params.Get("offset"),
			Names: params.Get("names"), Filter: params.Get("filter"),
		})
		writeStoreAnswer(r.Context(), w, answer, svcErr)
	}
}

func getHandler[T any](get func(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[T], *tidcommon.ServiceError)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		answer, svcErr := get(r.Context(), r.PathValue("id"), r.PathValue("name"))
		writeStoreAnswer(r.Context(), w, answer, svcErr)
	}
}

func createHandler[Req, T any](create func(ctx context.Context, gatewayID string,
	req Req) (*StoreAnswer[T], *tidcommon.ServiceError)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		r.Body = http.MaxBytesReader(w, r.Body, maxStoreBody)
		if !decodeBody(w, r, &req) {
			return
		}
		answer, svcErr := create(r.Context(), r.PathValue("id"), req)
		writeStoreAnswer(r.Context(), w, answer, svcErr)
	}
}

func setHandler[Req, T any](set func(ctx context.Context, gatewayID, name string,
	req Req) (*StoreAnswer[T], *tidcommon.ServiceError)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req Req
		r.Body = http.MaxBytesReader(w, r.Body, maxStoreBody)
		if !decodeBody(w, r, &req) {
			return
		}
		answer, svcErr := set(r.Context(), r.PathValue("id"), r.PathValue("name"), req)
		writeStoreAnswer(r.Context(), w, answer, svcErr)
	}
}

func deleteHandler(remove func(ctx context.Context, gatewayID,
	name string) (*StoreAnswer[struct{}], *tidcommon.ServiceError)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		answer, svcErr := remove(r.Context(), r.PathValue("id"), r.PathValue("name"))
		writeStoreAnswer(r.Context(), w, answer, svcErr)
	}
}

// writeStoreAnswer answers as the gateway did: with what it returned, or with its own refusal.
func writeStoreAnswer[T any](ctx context.Context, w http.ResponseWriter, answer *StoreAnswer[T],
	svcErr *tidcommon.ServiceError) {
	switch {
	case svcErr != nil:
		handleError(ctx, w, svcErr)
	case answer.Refusal != nil:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(answer.Refusal.Status)
		_, _ = w.Write(answer.Refusal.Body)
	case answer.Value == nil:
		w.WriteHeader(http.StatusNoContent)
	case answer.Created:
		sysutils.WriteSuccessResponse(ctx, w, http.StatusCreated, answer.Value)
	default:
		sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, answer.Value)
	}
}
