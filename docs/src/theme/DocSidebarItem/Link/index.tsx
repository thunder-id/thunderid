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

import Link from '@docusaurus/Link';
import {usePluginData} from '@docusaurus/useGlobalData';
import OriginalDocSidebarItemLink from '@theme-original/DocSidebarItem/Link';
import React from 'react';
import AndroidLogo from '@site/src/components/icons/AndroidLogo';
import ExpressLogo from '@site/src/components/icons/ExpressLogo';
import FlutterLogo from '@site/src/components/icons/FlutterLogo';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import JavaScriptLogo from '@site/src/components/icons/JavaScriptLogo';
import LangChainLogo from '@site/src/components/icons/LangChainLogo';
import NextLogo from '@site/src/components/icons/NextLogo';
import NodeLogo from '@site/src/components/icons/NodeLogo';
import NuxtLogo from '@site/src/components/icons/NuxtLogo';
import ReactLogo from '@site/src/components/icons/ReactLogo';
import VueLogo from '@site/src/components/icons/VueLogo';

interface PersonaPluginData {
  personaMap: Record<string, string>;
}

interface SidebarItem {
  docId?: string;
  href?: string;
  label?: string;
  className?: string;
  customProps?: {icon?: string};
  [key: string]: unknown;
}

type OriginalProps = Omit<React.ComponentProps<typeof OriginalDocSidebarItemLink>, 'item'> & {
  item: SidebarItem;
};

const TECH_LOGOS: Record<string, React.ReactElement> = {
  android: <AndroidLogo size={20} />,
  express: <ExpressLogo size={20} />,
  flutter: <FlutterLogo size={20} />,
  ios: <IOSLogo size={20} />,
  javascript: <JavaScriptLogo size={20} />,
  next: <NextLogo size={20} />,
  node: <NodeLogo size={20} />,
  nuxt: <NuxtLogo size={20} />,
  react: <ReactLogo size={20} />,
  vue: <VueLogo size={20} />,
  langchain: <LangChainLogo size={20} />,
};

export default function DocSidebarItemLink({item, ...rest}: OriginalProps): React.ReactElement {
  const {personaMap} = usePluginData('product-persona-plugin') as PersonaPluginData;
  const persona = item.docId ? personaMap[item.docId] : undefined;

  const iconKey = item.customProps?.icon;
  const logo = iconKey ? TECH_LOGOS[iconKey] : undefined;

  // For icon items render our own structure so item.label stays a plain string.
  if (logo) {
    const level = (rest as {level?: number}).level ?? 1;
    const activePath = (rest as {activePath?: string}).activePath ?? '';
    const href = item.href ?? '#';
    const isActive = activePath === href || activePath.startsWith(`${href}/`);
    const className = [
      'theme-doc-sidebar-item-link',
      `theme-doc-sidebar-item-link-level-${level}`,
      'menu__list-item',
      item.className,
      persona ? `sidebar-persona-${persona}` : '',
    ].filter(Boolean).join(' ');

    return (
      <li className={className}>
        <Link
          to={href}
          className={`menu__link${isActive ? ' menu__link--active' : ''}`}
          aria-current={isActive ? 'page' : undefined}
        >
          <span className="sidebar-tech-label">
            <span className="sidebar-tech-icon" aria-hidden="true" style={{filter: 'grayscale(1) opacity(0.75)'}}>{logo}</span>
            {item.label}
          </span>
        </Link>
      </li>
    );
  }

  let enrichedItem: SidebarItem = {...item};

  if (persona) {
    enrichedItem = {
      ...enrichedItem,
      className: `${enrichedItem.className ?? ''} sidebar-persona-${persona}`.trim(),
    };
  }

  return <OriginalDocSidebarItemLink {...rest} item={enrichedItem} />;
}
