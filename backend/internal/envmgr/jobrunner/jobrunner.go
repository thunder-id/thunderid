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

// Package jobrunner delivers work queued for the data planes a control plane pod can reach.
package jobrunner

import (
	"context"
	"time"

	"github.com/thunder-id/thunderid/internal/system/channel"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// jobSweepInterval is how often a pod looks for work left for it by another.
//
// This is a backstop, not the path a request takes: work is delivered immediately by the pod that
// accepted it whenever that pod holds the connection. It only matters for work queued by a pod that
// did not, so it is deliberately unhurried.
const jobSweepInterval = 15 * time.Second

// ConfigSealer adapts the server's configuration crypto to what the environment manager needs for a
// credential queued for a data plane. A credential waiting to be delivered is encrypted at rest with
// the same key the rest of the server's configuration secrets use.
type ConfigSealer struct {
	crypto common.ConfigCryptoProvider
}

// NewConfigSealer builds a ConfigSealer over the server's configuration crypto.
func NewConfigSealer(crypto common.ConfigCryptoProvider) ConfigSealer {
	return ConfigSealer{crypto: crypto}
}

// Seal encrypts a credential queued for a data plane.
func (s ConfigSealer) Seal(ctx context.Context, plaintext []byte) ([]byte, error) {
	return s.crypto.Encrypt(ctx, plaintext)
}

// Open decrypts a credential read back from the queue.
func (s ConfigSealer) Open(ctx context.Context, sealed []byte) ([]byte, error) {
	return s.crypto.Decrypt(ctx, sealed)
}

// Deliverer carries out work queued for a data plane this pod can reach.
type Deliverer interface {
	DeliverPending(ctx context.Context, dataPlaneID string) error
}

// Start sweeps for work queued for the data planes connected to this pod.
//
// A control plane pod can only speak to the data planes that dialed it, so a request accepted by a
// pod holding no connection is written down instead of sent. This is what picks it up: each pod
// looks only at the data planes it can actually reach, and the claim in the database is what keeps
// two pods from delivering the same thing.
func Start(ctx context.Context, deliverer Deliverer, channelServer *channel.Server) {
	if deliverer == nil || channelServer == nil {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DataPlaneJobWorker"))

	go func() {
		ticker := time.NewTicker(jobSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sweep(ctx, deliverer, channelServer, logger)
			}
		}
	}()
}

// sweep delivers one queued job for each connected data plane. One per sweep is deliberate: work for
// a data plane is delivered in order, so the next piece waits for this one to finish.
func sweep(ctx context.Context, deliverer Deliverer, channelServer *channel.Server,
	logger *log.Logger) {
	seen := make(map[string]bool)
	for _, conn := range channelServer.Connections() {
		if conn.ID == "" || seen[conn.ID] {
			continue
		}
		seen[conn.ID] = true

		// The sweep runs outside any request, so it carries the server's own authority rather than a
		// caller's: there is no caller.
		// The sweep runs outside any request, so it carries the server's own authority rather than a
		// caller's: there is no caller.
		if err := deliverer.DeliverPending(security.WithRuntimeContext(ctx), conn.ID); err != nil {
			logger.Warn(ctx, "Failed to deliver queued work to a data plane",
				log.String("dataPlaneId", conn.ID), log.Error(err))
		}
	}
}
