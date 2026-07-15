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

package main

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/revocationcache"
	"github.com/thunder-id/thunderid/internal/tokenstatus"
)

// statusListProducer obtains the signed Status List Token for a list id. The in-process Status List
// service satisfies it; this narrow view is all the in-process source needs.
type statusListProducer interface {
	Produce(ctx context.Context, listID string) (token string, ttl int, err error)
}

// inProcessStatusListSource is a revocationcache.StatusListSource that obtains the signed Status List
// Token from the local Status List service and decodes it. Wiring it here at the composition root keeps
// the revocation cache format-agnostic and decoupled from the Status List subsystem: a remote source
// would fetch the token over HTTP from the /statuslists endpoint instead, reusing
// tokenstatus.DecodeStatusListToken unchanged.
type inProcessStatusListSource struct {
	producer statusListProducer
}

// newStatusListSource builds an in-process status list source that feeds the Resource Server revocation
// cache from the local Status List service.
func newStatusListSource(producer statusListProducer) revocationcache.StatusListSource {
	return &inProcessStatusListSource{producer: producer}
}

// Fetch obtains the Status List Token for the list identified by uri and decodes it into the recorded
// entries keyed by index and the capacity its bit array covers. found is false when the list does not
// exist, so the cache can fail closed on an unresolvable reference rather than treating an empty result
// as an all-VALID list.
func (s *inProcessStatusListSource) Fetch(
	ctx context.Context, uri string,
) (map[int64]int, int64, bool, error) {
	listID, err := tokenstatus.ListIDFromURI(uri)
	if err != nil {
		return nil, 0, false, err
	}
	token, _, err := s.producer.Produce(ctx, listID)
	if err != nil {
		if errors.Is(err, tokenstatus.ErrListNotFound) {
			return nil, 0, false, nil
		}
		return nil, 0, false, err
	}
	statuses, capacity, err := tokenstatus.DecodeStatusListToken(token)
	if err != nil {
		return nil, 0, false, err
	}
	return statuses, capacity, true, nil
}
