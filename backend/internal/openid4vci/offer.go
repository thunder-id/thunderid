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

package openid4vci

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// credentialOfferScheme is the URI scheme wallets handle for credential offers.
const credentialOfferScheme = "openid-credential-offer://" //nolint:gosec

// defaultOfferTTL bounds how long a stored credential offer is retrievable.
const defaultOfferTTL = 5 * time.Minute

// GenerateCredentialOffer builds and stores an issuer-initiated credential offer for configID and
// returns it with the openid-credential-offer:// deep link (offer by reference).
func (s *service) GenerateCredentialOffer(
	ctx context.Context, configID string,
) (map[string]interface{}, string, error) {
	cred, svcErr := s.creds.GetCredentialConfigurationByHandle(ctx, configID)
	if svcErr != nil {
		return nil, "", fmt.Errorf("%w: %s", ErrUnsupportedCredential, configID)
	}

	offer := map[string]interface{}{
		"credential_issuer":            s.cfg.CredentialIssuer,
		"credential_configuration_ids": []string{cred.Handle},
		"grants": map[string]interface{}{
			"authorization_code": map[string]interface{}{},
		},
	}

	id, err := randomToken()
	if err != nil {
		return nil, "", fmt.Errorf("%w: failed to generate offer id: %w", ErrIssuance, err)
	}
	rec := &offerRecord{ID: id, Offer: offer, ExpiresAt: time.Now().Add(defaultOfferTTL)}
	if err := s.store.SaveOffer(ctx, rec); err != nil {
		return nil, "", fmt.Errorf("%w: failed to store credential offer: %w", ErrIssuance, err)
	}

	offerURI := s.cfg.BaseURL + credentialOfferPath + "/" + id
	deepLink := credentialOfferScheme + "?credential_offer_uri=" + url.QueryEscape(offerURI)
	return offer, deepLink, nil
}

// GetCredentialOffer returns a previously stored issuer-initiated credential offer by
// id, so a wallet can resolve the credential_offer_uri.
func (s *service) GetCredentialOffer(ctx context.Context, id string) (map[string]interface{}, error) {
	rec, ok := s.store.GetOffer(ctx, id)
	if !ok || rec == nil {
		return nil, fmt.Errorf("%w: unknown credential offer", ErrUnsupportedCredential)
	}
	if time.Now().After(rec.ExpiresAt) {
		return nil, fmt.Errorf("%w: credential offer expired", ErrUnsupportedCredential)
	}
	return rec.Offer, nil
}
