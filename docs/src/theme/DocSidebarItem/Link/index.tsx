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

import {usePluginData} from '@docusaurus/useGlobalData';
import OriginalDocSidebarItemLink from '@theme-original/DocSidebarItem/Link';
import React from 'react';
import type {Maturity} from '@site/plugins/maturityPlugin';

interface PersonaPluginData {
  personaMap: Record<string, string>;
}

interface MaturityPluginData {
  maturityMap: Record<string, Maturity>;
}

interface FeatureFlagPluginData {
  hiddenDocIds: string[];
}

interface SidebarItem {
  docId?: string;
  className?: string;
  [key: string]: unknown;
}

type OriginalProps = Omit<React.ComponentProps<typeof OriginalDocSidebarItemLink>, 'item'> & {
  item: SidebarItem;
};

export default function DocSidebarItemLink({item, ...rest}: OriginalProps): React.ReactElement | null {
  const {personaMap} = usePluginData('product-persona-plugin') as PersonaPluginData;
  const {maturityMap} = usePluginData('product-maturity-plugin') as MaturityPluginData;
  const {hiddenDocIds} = usePluginData('product-feature-flag-plugin') as FeatureFlagPluginData;

  if (item.docId && hiddenDocIds.includes(item.docId)) {
    return null;
  }

  const persona = item.docId ? personaMap[item.docId] : undefined;
  const maturity = item.docId ? maturityMap[item.docId] : undefined;

  const extraClasses = [
    persona && `sidebar-persona-${persona}`,
    maturity && `sidebar-maturity-${maturity}`,
  ]
    .filter(Boolean)
    .join(' ');

  if (extraClasses) {
    const enrichedItem: SidebarItem = {
      ...item,
      className: `${item.className ?? ''} ${extraClasses}`.trim(),
    };
    return <OriginalDocSidebarItemLink {...rest} item={enrichedItem} />;
  }

  return <OriginalDocSidebarItemLink {...rest} item={item} />;
}
