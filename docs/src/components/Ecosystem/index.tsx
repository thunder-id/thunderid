/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box} from '@wso2/oxygen-ui';
import {JSX, useMemo, useState} from 'react';
import {ECOSYSTEM_ITEMS, EcosystemCategory} from './data';
import EcosystemCTA from './EcosystemCTA';
import EcosystemGrid from './EcosystemGrid';
import EcosystemHero from './EcosystemHero';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

export default function EcosystemPage(): JSX.Element {
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState<'all' | EcosystemCategory>('all');
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;

  const items = useMemo(
    () => ECOSYSTEM_ITEMS.map((item) => ({...item, description: item.description.replace(/\{\{ProductName\}\}/g, productName)})),
    [productName],
  );

  const filteredItems = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((item) => {
      if (category !== 'all' && item.category !== category) return false;
      if (!q) return true;
      return (
        item.name.toLowerCase().includes(q) ||
        item.packageName.toLowerCase().includes(q) ||
        item.description.toLowerCase().includes(q)
      );
    });
  }, [items, query, category]);

  return (
    <Box>
      <EcosystemHero query={query} onQueryChange={setQuery} category={category} onCategoryChange={setCategory} />
      <EcosystemGrid query={query} items={filteredItems} />
      <EcosystemCTA />
    </Box>
  );
}
