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

import type {ConnectionCardModel, ConnectionInstance, ConnectionVendorMeta} from '../models/connection';

/**
 * Merge the flat GET /connections instance list with the FE vendor-meta catalog into the list
 * of cards the listing grid renders.
 *
 * - every configured instance → one card titled by the instance name (each opens its own
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
  const instancesByType: Record<string, ConnectionInstance[]> = {};
  for (const instance of instances) {
    (instancesByType[instance.type] ??= []).push(instance);
  }

  const cards: ConnectionCardModel[] = [];

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
