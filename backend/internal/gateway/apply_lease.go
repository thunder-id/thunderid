// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// leaseTimeout is how long an apply may hold a gateway's lease before another takes it over: as long
// as reading the store and the import together may take, so a lease outlives no apply that is still
// running, only one whose node stopped.
const leaseTimeout = storeTimeout + importTimeout

// leasePoll is how often an apply waiting for a gateway's lease looks again, and leaseWait how long
// it waits in all, as long as an import may take. Variables so a test can shorten them.
var (
	leasePoll = 500 * time.Millisecond
	leaseWait = importTimeout
)

// lease takes the gateway's lease for an apply, so one apply or revert runs at a time per gateway
// across every node of this plane, and returns what releases it and the token that holds it.
//
// While another apply holds the lease, an apply of the latest version does not wait: it asks for the
// latest version to be applied again once the running one finishes, and reports itself queued. A
// burst of changes then costs at most one more apply, which sends the latest version at that time,
// however many were asked for meanwhile. Any other apply, of an older version or a revert, waits as
// long as an import may take and is refused if the lease is still held then, so a user's explicit
// choice is never folded into "latest". A lease held longer than any apply can run is taken over by
// the claim itself, on the database's clock.
func (s *versionService) lease(ctx context.Context, gatewayID string, coalesce bool) (func(), string, bool,
	*tidcommon.ServiceError) {
	token := uuid.New().String()
	deadline := time.Now().Add(leaseWait)
	for {
		claimed, err := s.versions.ClaimLease(ctx, gatewayID, token)
		if err != nil {
			s.logger.Error(ctx, "Failed to claim the gateway's apply lease", log.Error(err))
			return nil, "", false, &tidcommon.InternalServerError
		}
		if claimed {
			return s.releaser(ctx, gatewayID, token), token, false, nil
		}
		if coalesce {
			queued, err := s.versions.MarkPendingLatest(ctx, gatewayID, actorOf(ctx))
			if err != nil {
				s.logger.Error(ctx, "Failed to queue the latest version behind the running apply", log.Error(err))
				return nil, "", false, &tidcommon.InternalServerError
			}
			if queued {
				s.logger.Info(ctx, "Queued the latest version behind the running apply",
					log.String("gatewayId", gatewayID))
				return nil, "", true, nil
			}
			// Released between the claim and the ask; claim again.
		} else if time.Now().After(deadline) {
			return nil, "", false, &ErrorApplyInProgress
		}
		select {
		case <-ctx.Done():
			return nil, "", false, &ErrorApplyInProgress
		case <-time.After(leasePoll):
		}
	}
}

// releaser gives the lease back and, when the latest version was asked for meanwhile, applies it
// once more on this node, for whoever asked.
func (s *versionService) releaser(ctx context.Context, gatewayID, token string) func() {
	return func() {
		// The release outlives the request that applied, which may be gone by now.
		ctx := context.WithoutCancel(ctx)
		if err := s.versions.ReleaseLease(ctx, gatewayID, token); err != nil {
			s.logger.Error(ctx, "Failed to release the gateway's apply lease", log.String("gatewayId", gatewayID),
				log.Error(err))
		}
		by, pending, err := s.versions.TakePendingLatest(ctx, gatewayID)
		if err != nil {
			s.logger.Error(ctx, "Failed to read whether the latest version is to be applied again",
				log.String("gatewayId", gatewayID), log.Error(err))
			return
		}
		if pending {
			go s.applyLatestAgain(withActor(security.WithRuntimeContext(ctx), by), gatewayID)
		}
	}
}

// applyLatestAgain applies the latest version to a gateway, as asked for while another apply ran.
func (s *versionService) applyLatestAgain(ctx context.Context, gatewayID string) {
	ctx, cancel := context.WithTimeout(ctx, leaseTimeout)
	defer cancel()
	result, svcErr := s.Apply(ctx, gatewayID, ApplyRequest{})
	switch {
	case svcErr != nil:
		s.logger.Warn(ctx, "The latest version queued behind an apply could not be applied",
			log.String("gatewayId", gatewayID), log.String("code", svcErr.Code))
	case result.Queued:
		s.logger.Info(ctx, "The latest version is queued once more behind another apply",
			log.String("gatewayId", gatewayID))
	default:
		s.logger.Info(ctx, "Applied the latest version queued behind an apply", log.String("gatewayId", gatewayID),
			log.String("version", result.Diff.ToVersion))
	}
}

// removalConfirmationLimit reads how many resources an apply may remove without being confirmed.
func removalConfirmationLimit() int {
	if !config.IsServerRuntimeInitialized() {
		return 0
	}
	return config.GetServerRuntime().Config.Gateway.RemovalConfirmationLimit()
}
