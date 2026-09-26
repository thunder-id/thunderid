// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	"github.com/thunder-id/thunderid/internal/role"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/outboundauthn"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// rbacEngine evaluates authorization requests using the role service.
type rbacEngine struct {
	roleService role.RoleServiceInterface
}

// evaluationGroup collects equivalent evaluations for one role-service lookup.
type evaluationGroup struct {
	subject          Subject
	resourceServerID string
	permissions      []string
	indexes          []int
}

// AuthZENPDPConfig configures an AuthZEN PDP access-evaluation endpoint.
type AuthZENPDPConfig struct {
	ResourceType             string
	Endpoint                 string
	BatchEndpoint            string
	Timeout                  time.Duration
	RetryCount               int
	SubjectAttributeMappings []authzenpdp.SubjectAttributeMapping
}

// authZENPDPEngine evaluates access requests through an external AuthZEN PDP.
type authZENPDPEngine struct {
	connectionService authZENPDPConnectionService
	client            httpservice.HTTPClientInterface
	logger            *log.Logger
}

// authZENPDPConnectionService resolves AuthZEN PDP connection settings by ID.
type authZENPDPConnectionService interface {
	GetAuthZENPDP(context.Context, string) (*authzenpdp.AuthZENPDPConnection, *tidcommon.ServiceError)
}

// authZENPDPSettings holds normalized settings used for one PDP evaluation.
type authZENPDPSettings struct {
	resourceType             string
	endpoint                 string
	batchEndpoint            string
	retryCount               int
	subjectAttributeMappings []authzenpdp.SubjectAttributeMapping
	timeout                  time.Duration
	authenticator            outboundauthn.RequestAuthenticator
}

// authZENPDPBatch holds grouped evaluations and their original indexes.
type authZENPDPBatch struct {
	request AccessEvaluationsRequest
	indexes []int
}

// authZENPDPBatchKey identifies evaluations that can be sent in one batch request.
type authZENPDPBatchKey struct {
	connectionID string
	resourceType string
}

const (
	subjectGroupsProperty = "groups"
	// DelegatedResourceIDProperty carries an opaque resource instance ID to the PDP engine.
	DelegatedResourceIDProperty = "__delegatedResourceId"
)
