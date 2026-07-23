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

import {Collapsible} from '@docusaurus/theme-common';
import type {Props} from '@theme/DocSidebarItem/Category';
import DocSidebarItems from '@theme/DocSidebarItems';
import OriginalDocSidebarItemCategory from '@theme-original/DocSidebarItem/Category';
import {Box} from '@wso2/oxygen-ui';
import {Bot, MonitorSmartphone, Server} from '@wso2/oxygen-ui-icons-react';
import React from 'react';
import {ConnectType, applyConnectType, useConnectType} from '@site/src/utils/connectType';

type OriginalProps = Props;

// The "What are you building?" options (Application / AI Agent / MCP Server)
// render as always-visible cards. Only the one matching the shared connect-type
// is expanded; clicking a card sets the connect-type, which collapses the
// others and stays in sync with the docs-home selector. MCP Server is a
// disabled card while its quickstarts are still coming.
const SECTIONS: Record<ConnectType, {Icon: typeof Bot; comingSoon: boolean}> = {
  app: {Icon: MonitorSmartphone, comingSoon: false},
  agent: {Icon: Bot, comingSoon: false},
  mcp: {Icon: Server, comingSoon: true},
};

function connectTypeFromClassName(className: string | undefined): ConnectType | undefined {
  if (!className) return undefined;
  if (className.includes('connect-section--app')) return 'app';
  if (className.includes('connect-section--agent')) return 'agent';
  if (className.includes('connect-section--mcp')) return 'mcp';
  return undefined;
}

const cardSx = {
  alignItems: 'center',
  background: 'rgba(255, 255, 255, 0.07)',
  border: '1px solid rgba(255, 255, 255, 0.22)',
  borderRadius: '10px',
  color: 'var(--ifm-font-color-base)',
  cursor: 'pointer',
  display: 'flex',
  fontSize: '0.9rem',
  fontWeight: 600,
  gap: '0.65rem',
  margin: '0.1rem var(--ifm-menu-link-padding-horizontal, 0.75rem) 0.5rem',
  padding: '0.65rem 0.75rem',
  textAlign: 'left',
  transition: 'border-color 0.2s ease, box-shadow 0.2s ease',
  width: 'calc(100% - 2 * var(--ifm-menu-link-padding-horizontal, 0.75rem))',
  '&:hover': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 55%, transparent)',
    boxShadow: '0 0 0 3px color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)',
  },
  '[data-theme="light"] &': {
    background: 'rgba(0, 0, 0, 0.04)',
    borderColor: 'rgba(0, 0, 0, 0.12)',
  },
};

const cardActiveSx = {
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 55%, transparent)',
  boxShadow: '0 0 0 3px color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)',
  '[data-theme="light"] &': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 55%, transparent)',
  },
};

const cardDisabledSx = {
  cursor: 'default',
  opacity: 0.5,
  '&:hover': {
    borderColor: 'rgba(255, 255, 255, 0.22)',
    boxShadow: 'none',
  },
  '[data-theme="light"] &:hover': {borderColor: 'rgba(0, 0, 0, 0.12)'},
};

const iconBoxSx = {
  alignItems: 'center',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 18%, transparent)',
  borderRadius: '7px',
  color: 'var(--ifm-color-primary)',
  display: 'inline-flex',
  flexShrink: 0,
  height: '2rem',
  justifyContent: 'center',
  width: '2rem',
};

const badgeSx = {
  background: 'color-mix(in srgb, var(--ifm-color-emphasis-400) 30%, transparent)',
  borderRadius: '20px',
  color: 'var(--ifm-color-content-secondary)',
  fontSize: '0.6rem',
  fontWeight: 600,
  letterSpacing: '0.04em',
  padding: '0.1rem 0.4rem',
  textTransform: 'uppercase',
};

function ConnectSection({item, ...rest}: OriginalProps): React.ReactElement {
  const {items, label, className} = item;
  const type = connectTypeFromClassName(className)!;
  const {Icon, comingSoon} = SECTIONS[type];
  const active = useConnectType();
  const expanded = !comingSoon && active === type;

  return (
    <li className="menu__list-item">
      <Box
        component={comingSoon ? 'div' : 'button'}
        type={comingSoon ? undefined : 'button'}
        aria-expanded={comingSoon ? undefined : expanded}
        onClick={comingSoon ? undefined : () => applyConnectType(type)}
        sx={{...cardSx, ...(expanded ? cardActiveSx : {}), ...(comingSoon ? cardDisabledSx : {})}}
      >
        <Box className="cts-opt-icon" component="span" sx={iconBoxSx}>
          <Icon aria-hidden size={20} />
        </Box>
        <Box component="span" sx={{flex: 1}}>{label}</Box>
        {comingSoon && <Box component="span" sx={badgeSx}>Soon</Box>}
      </Box>

      {!comingSoon && (
        <Collapsible lazy as="ul" className="menu__list" collapsed={!expanded}>
          <DocSidebarItems
            items={items}
            tabIndex={expanded ? 0 : -1}
            activePath={rest.activePath}
            onItemClick={rest.onItemClick}
            level={rest.level + 1}
          />
        </Collapsible>
      )}
    </li>
  );
}

export default function DocSidebarItemCategory({item, ...rest}: OriginalProps): React.ReactElement {
  if (connectTypeFromClassName(item.className)) {
    return <ConnectSection item={item} {...rest} />;
  }
  return <OriginalDocSidebarItemCategory item={item} {...rest} />;
}
