// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// captureSubmittedCredentials hands the credentials a new user was created with to the value
// capturer.
//
// They are read from the attributes as submitted, because creation strips them from what it returns
// and stores only their hash. Which attributes are credentials is the user type's to say, and it is
// asked only when there is a capturer to hand them to.
func (us *userService) captureSubmittedCredentials(ctx context.Context, user *providers.User,
	submitted json.RawMessage) {
	if us.valueCapturer == nil || us.entityTypeService == nil || len(submitted) == 0 {
		return
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(submitted, &attrs); err != nil {
		return
	}
	declared, svcErr := us.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, user.Type,
		entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).Warn(ctx,
			"Failed to read which attributes are credentials, so none were captured",
			log.MaskedString(log.LoggerKeyUserID, user.ID))
		return
	}
	credentials := make(map[string]string, len(declared))
	for _, attribute := range declared {
		if value, ok := attrs[attribute.Attribute].(string); ok {
			credentials[attribute.Attribute] = value
		}
	}
	us.captureCredentials(ctx, user, submitted, credentials)
}

// captureCredentials hands a user's credentials to the value capturer, in the shape its exporter
// reads back: the attributes, which name the user, and the credentials beside them. The export names
// each credential after the username, so a user without one has nothing captured, as it has nothing
// exported.
func (us *userService) captureCredentials(ctx context.Context, user *providers.User,
	attributes json.RawMessage, credentials map[string]string) {
	if us.valueCapturer == nil || len(credentials) == 0 {
		return
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(attributes, &attrs); err != nil {
		return
	}
	if username, _ := attrs["username"].(string); username == "" {
		return
	}
	resource := &userDeclarativeResource{
		ID:          user.ID,
		Type:        user.Type,
		OUID:        user.OUID,
		Attributes:  map[string]interface{}{},
		Credentials: make(map[string]interface{}, len(credentials)),
	}
	for name, value := range attrs {
		if _, isCredential := credentials[name]; !isCredential {
			resource.Attributes[name] = value
		}
	}
	for name, value := range credentials {
		resource.Credentials[name] = value
	}
	us.valueCapturer.CaptureValues(ctx, resourceTypeUser, resource)
}
