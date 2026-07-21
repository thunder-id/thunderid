/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import {
  ConnectionTypes,
  type ConnectionCardModel,
  type ConnectionInstance,
  type ConnectionVendorMeta,
} from '../models/connection';

/** i18n key for the trusted issuer card's description, resolved by the rendering component. */
const TRUSTED_IDP_DESCRIPTION_KEY = 'connections:vendor.trustedIdp.description';

/**
 * Whether a connection instance is a trusted issuer — a trust-only OIDC instance used for ID-JAG
 * identity assertion consumption — rather than a plain federation OIDC connection. Trusted
 * issuers always carry `idJagEnabled` (`true` or `false`); plain federation OIDC connections
 * leave it `undefined`.
 */
function isTrustedIdpInstance(instance: ConnectionInstance): boolean {
  return instance.type === ConnectionTypes.OIDC && instance.idJagEnabled !== undefined;
}

/**
 * Merge the flat GET /connections instance list with the FE vendor-meta catalog into the list
 * of cards the listing grid renders.
 *
 * - OIDC instances that are trusted issuers (`idJagEnabled` present) are pulled out first and
 *   rendered as their own card variant, before the normal per-vendor grouping below — each opens
 *   the trusted issuer detail page rather than the generic connection detail page.
 * - every other configured instance → one card titled by the instance name (each opens its own
 *   detail page), for branded and custom vendors alike.
 * - branded vendors with no instances → one unconfigured card that opens the configure wizard.
 * - custom vendors with no instances → no card (creation goes through the custom-connection
 *   wizard entry point).
 * - coming-soon vendors → one static, non-interactive card.
 * - instances whose type has no vendor meta (e.g. custom SMS gateway senders) are not rendered.
 *
 * Pure function — no i18n, no hooks — so it is trivially unit-testable. The card carries a
 * `descriptionKey`; the rendering component resolves it.
 */
export default function buildConnectionCards(
  instances: ConnectionInstance[],
  vendorMetas: ConnectionVendorMeta[],
): ConnectionCardModel[] {
  const trustedIdpCards: ConnectionCardModel[] = instances.filter(isTrustedIdpInstance).map(
    (instance): ConnectionCardModel => ({
      id: `trusted-idp:${instance.id}`,
      vendorKey: 'trusted-idp',
      backendType: ConnectionTypes.OIDC,
      displayName: instance.name,
      descriptionKey: TRUSTED_IDP_DESCRIPTION_KEY,
      logo: <ShieldCheck />,
      categories: ['trusted-idp'],
      status: 'configured',
      comingSoon: false,
      navTarget: `/trusted-issuers/${instance.id}`,
    }),
  );

  const remainingInstances: ConnectionInstance[] = instances.filter((instance) => !isTrustedIdpInstance(instance));

  const instancesByType: Record<string, ConnectionInstance[]> = {};
  for (const instance of remainingInstances) {
    (instancesByType[instance.type] ??= []).push(instance);
  }

  const cards: ConnectionCardModel[] = [...trustedIdpCards];

  for (const meta of vendorMetas) {
    if ((meta.presentation === 'branded' || meta.presentation === 'custom') && meta.backendType) {
      const vendorInstances: ConnectionInstance[] = instancesByType[meta.backendType] ?? [];
      for (const instance of vendorInstances) {
        cards.push({
          id: `${meta.key}:${instance.id}`,
          vendorKey: meta.key,
          backendType: meta.backendType,
          displayName: instance.name,
          descriptionKey: meta.descriptionKey,
          logo: meta.logo,
          categories: meta.categories,
          status: 'configured',
          comingSoon: false,
          navTarget: `/connections/${meta.backendType}/${instance.id}`,
        });
      }

      if (meta.presentation === 'branded' && vendorInstances.length === 0) {
        cards.push({
          id: meta.key,
          vendorKey: meta.key,
          backendType: meta.backendType,
          displayName: meta.displayName,
          descriptionKey: meta.descriptionKey,
          logo: meta.logo,
          categories: meta.categories,
          status: 'not-configured',
          comingSoon: false,
          navTarget: `/connections/${meta.backendType}/configure`,
        });
      }
      continue;
    }

    // coming-soon
    cards.push({
      id: meta.key,
      vendorKey: meta.key,
      backendType: meta.backendType,
      displayName: meta.displayName,
      descriptionKey: meta.descriptionKey,
      logo: meta.logo,
      categories: meta.categories,
      status: 'not-configured',
      comingSoon: true,
      navTarget: null,
    });
  }

  return cards;
}
