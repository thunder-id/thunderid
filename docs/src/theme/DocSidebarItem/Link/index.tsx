// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import {usePluginData} from '@docusaurus/useGlobalData';
import OriginalDocSidebarItemLink from '@theme-original/DocSidebarItem/Link';
import {AndroidLogo, FlutterLogo, LangChainLogo} from '@thunderid/components';
import React from 'react';
import ExpressLogo from '@site/src/components/icons/ExpressLogo';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import JavaScriptLogo from '@site/src/components/icons/JavaScriptLogo';
import NextLogo from '@site/src/components/icons/NextLogo';
import NodeLogo from '@site/src/components/icons/NodeLogo';
import NuxtLogo from '@site/src/components/icons/NuxtLogo';
import PythonLogo from '@site/src/components/icons/PythonLogo';
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
  python: <PythonLogo size={20} />,
  react: <ReactLogo size={20} />,
  vue: <VueLogo size={20} />,
  langchain: <LangChainLogo size={20} />,
};

export default function DocSidebarItemLink({item, ...rest}: OriginalProps): React.ReactElement {
  const {personaMap} = usePluginData('product-persona-plugin') as PersonaPluginData;
  const persona = item.docId ? personaMap[item.docId] : undefined;

  const iconKey = item.customProps?.icon;
  const googleAdkLogo = useBaseUrl('/assets/images/agent/google-adk.svg');
  const crewaiLogo = useBaseUrl('/assets/images/agent/crewai.svg');
  const logos: Record<string, React.ReactElement> = {
    ...TECH_LOGOS,
    'google-adk': <img src={googleAdkLogo} alt="" width={20} height={20} />,
    crewai: <img src={crewaiLogo} alt="" width={20} height={20} />,
    vercel: <svg viewBox="0 0 512 512" fill="currentColor" width={20} height={20} aria-hidden="true"><path d="M256 48 496 464H16Z" /></svg>,
  };
  const logo = iconKey ? logos[iconKey] : undefined;

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
