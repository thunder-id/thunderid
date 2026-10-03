// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// captureTimeout bounds one capture, every value of one resource together. The capture runs inside
// the write that triggered it, so a gateway that does not answer must not hold that write for long.
const captureTimeout = 10 * time.Second

// placeholderValuer computes the values an export of one resource refers to by name.
type placeholderValuer interface {
	PlaceholderValues(ctx context.Context, resourceType string, resource interface{}) (
		map[string]string, map[string]string, error)
}

// ValueCapture writes the values a resource's export refers to into the default gateway's store,
// under the names the export writes.
//
// A control plane exports references rather than values, and the gateway a version is applied to
// fills each reference from its own store at import. Something has to put the value there. Whoever
// writes a resource is the only one holding all of it, a generated client secret above all, which
// is hashed on write and never read back. So the value is taken from the write, as it happens.
//
// Only the default gateway receives values. There is no telling which other gateway a value is meant
// for: another may hold its own value under the same name on purpose.
//
// It is built before the services that call it and bound once the export and the gateway store
// exist, because the export is built from those services' exporters. A capture before the binding
// does nothing. The binding happens during startup, before any request is served, so it is not
// guarded.
type ValueCapture struct {
	values   placeholderValuer
	gateways storeInterface
	versions VersionServiceInterface
	logger   *log.Logger
}

// NewValueCapture returns a capture that does nothing until Initialize binds it.
func NewValueCapture() *ValueCapture {
	return &ValueCapture{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GatewayValueCapture")),
	}
}

func (c *ValueCapture) bind(values placeholderValuer, gateways storeInterface, versions VersionServiceInterface) {
	c.values = values
	c.gateways = gateways
	c.versions = versions
}

// CaptureValues writes the values the resource's export would refer to into the default gateway's
// store, variables and secrets each to its own collection.
//
// It never fails the caller. A value that does not arrive is logged by name, never by value, and the
// apply that later needs it reports it missing, which is where it can be set by hand.
//
// It is synchronous on purpose. An administrator who writes a resource and then captures and applies
// a version expects the value to be there; written in the background, it could arrive after the
// apply that needed it. The caller's cancellation is dropped, so a client that hangs up once its
// write has succeeded does not cut the capture short, and captureTimeout bounds it instead.
func (c *ValueCapture) CaptureValues(ctx context.Context, resourceType string, resource interface{}) {
	if c == nil || c.values == nil {
		return
	}
	// Detached and bounded from the start, so working out the values and finding the gateway are as
	// safe from a client that hangs up as the writes are, and the whole capture shares one bound.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), captureTimeout)
	defer cancel()

	variables, secrets, err := c.values.PlaceholderValues(ctx, resourceType, resource)
	if err != nil {
		c.logger.Warn(ctx, "Failed to work out the values a resource refers to, so none were captured",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if len(variables) == 0 && len(secrets) == 0 {
		return
	}

	gw, err := c.defaultGateway(ctx)
	if err != nil {
		c.logger.Warn(ctx, "Failed to find the default gateway, so no values were captured",
			log.String("resourceType", resourceType), log.Error(err))
		return
	}
	if gw == nil {
		c.logger.Debug(ctx, "No gateway is the default, so no values were captured",
			log.String("resourceType", resourceType))
		return
	}

	c.write(ctx, gw.ID, collectionVariables, variables)
	c.write(ctx, gw.ID, collectionSecrets, secrets)
}

// defaultGateway returns the default gateway, or nil when none is.
func (c *ValueCapture) defaultGateway(ctx context.Context) (*Gateway, error) {
	gateways, err := c.gateways.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range gateways {
		if gateways[i].IsDefault {
			return &gateways[i], nil
		}
	}
	return nil, nil
}

// storeValue is the body a gateway's store takes to set a value.
type storeValue struct {
	Value string `json:"value"`
}

// write sets each value in a collection of the gateway's store.
//
// A PUT is used rather than a POST because it sets the value whether or not the name is held: a
// resource written again, or a secret regenerated, replaces what the gateway held under that name.
// The call goes through the same forwarding an administrator's write to the store does, so the key
// is presented the same way.
func (c *ValueCapture) write(ctx context.Context, gatewayID, collection string, values map[string]string) {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		body, err := json.Marshal(storeValue{Value: values[name]})
		if err != nil {
			c.logger.Warn(ctx, "Failed to encode a captured value", log.String("collection", collection),
				log.String("name", name), log.Error(err))
			continue
		}
		answer, svcErr := c.versions.ForwardStore(ctx, gatewayID, StoreCall{
			Method: http.MethodPut,
			Path:   "/" + collection + "/" + url.PathEscape(name),
			Body:   body,
		})
		if svcErr != nil {
			c.logger.Warn(ctx, "Failed to capture a value in the default gateway's store",
				log.String("gatewayId", gatewayID), log.String("collection", collection),
				log.String("name", name), log.String("error", svcErr.Error.DefaultValue))
			continue
		}
		if answer.Status != http.StatusOK && answer.Status != http.StatusCreated {
			c.logger.Warn(ctx, "The default gateway's store refused a captured value",
				log.String("gatewayId", gatewayID), log.String("collection", collection),
				log.String("name", name), log.Int("status", answer.Status))
		}
	}
}
