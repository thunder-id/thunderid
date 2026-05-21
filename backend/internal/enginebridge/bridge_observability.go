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

package enginebridge

import (
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/internal/system/observability/publisher"
	"github.com/thunder-id/thunderid/internal/system/observability/subscriber"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type observabilityBridge struct {
	provider thunderidengine.ObservabilityProvider
}

func newObservabilityBridge(provider thunderidengine.ObservabilityProvider) *observabilityBridge {
	return &observabilityBridge{provider: provider}
}

func (b *observabilityBridge) PublishEvent(evt *event.Event) {
	if b.provider == nil || evt == nil {
		return
	}
	b.provider.PublishEvent(toPublicEvent(evt))
}

func (b *observabilityBridge) IsEnabled() bool {
	if b.provider == nil {
		return false
	}
	return b.provider.IsEnabled()
}

func (b *observabilityBridge) GetConfig() *config.ObservabilityConfig {
	return nil
}

func (b *observabilityBridge) GetPublisher() publisher.CategoryPublisherInterface {
	return nil
}

func (b *observabilityBridge) GetActiveSubscribers() []subscriber.SubscriberInterface {
	return nil
}

func (b *observabilityBridge) Shutdown() {}

var _ observability.ObservabilityServiceInterface = (*observabilityBridge)(nil)
