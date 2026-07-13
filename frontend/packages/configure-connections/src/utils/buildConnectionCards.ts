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

import type {
  ConnectionCardModel,
  ConnectionInstanceSummary,
  ConnectionTypeSummary,
  ConnectionVendorMeta,
} from '../models/connection';

/**
 * Merge connection type summaries, the FE vendor-meta catalog, and per-type instances into the
 * flat list of cards the listing grid renders.
 *
 * - branded vendors → one singleton card; a configured card opens its detail page, an
 *   unconfigured one opens the configure wizard.
 * - custom vendors → one card per configured instance (each opens its own detail page).
 * - coming-soon vendors → one static, non-interactive card.
 *
 * Pure function — no i18n, no hooks — so it is trivially unit-testable. The card carries a
 * `descriptionKey`; the rendering component resolves it.
 */
export default function buildConnectionCards(
  summaries: ConnectionTypeSummary[],
  vendorMetas: ConnectionVendorMeta[],
  instancesByType: Record<string, ConnectionInstanceSummary[]>,
): ConnectionCardModel[] {
  const cards: ConnectionCardModel[] = [];

  for (const meta of vendorMetas) {
    if (meta.presentation === 'branded' && meta.backendType) {
      const summary: ConnectionTypeSummary | undefined = summaries.find((s) => s.type === meta.backendType);
      const configured: boolean = summary?.configured ?? false;

      cards.push({
        id: meta.key,
        vendorKey: meta.key,
        backendType: meta.backendType,
        displayName: meta.displayName,
        descriptionKey: meta.descriptionKey,
        logo: meta.logo,
        categories: meta.categories,
        status: configured ? 'configured' : 'not-configured',
        comingSoon: false,
        navTarget: configured ? `/connections/${meta.backendType}` : `/connections/${meta.backendType}/configure`,
      });
      continue;
    }

    if (meta.presentation === 'custom' && meta.backendType) {
      const instances: ConnectionInstanceSummary[] = instancesByType[meta.backendType] ?? [];
      for (const instance of instances) {
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
