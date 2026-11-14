// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import type {PropSidebarItem} from '@docusaurus/plugin-content-docs';
import {isActiveSidebarItem} from '@docusaurus/plugin-content-docs/client';
import {Collapsible} from '@docusaurus/theme-common';
import type {Props} from '@theme/DocSidebarItem/Category';
import DocSidebarItems from '@theme/DocSidebarItems';
import OriginalDocSidebarItemCategory from '@theme-original/DocSidebarItem/Category';
import {Box} from '@wso2/oxygen-ui';
import {Bot, ChevronDown, MonitorSmartphone, Server} from '@wso2/oxygen-ui-icons-react';
import React, {useId, useRef, useState} from 'react';
import {ConnectType, applyConnectType, useConnectType} from '@site/src/utils/connectType';

type OriginalProps = Props;

// The "What are you building?" options (Application / AI Agent / MCP)
// render as always-visible cards. Only the one matching the shared connect-type
// is expanded; clicking a card sets the connect-type, which collapses the
// others and stays in sync with the docs-home selector.
const SECTIONS: Record<ConnectType, {Icon: typeof Bot; comingSoon: boolean}> = {
  app: {Icon: MonitorSmartphone, comingSoon: false},
  agent: {Icon: Bot, comingSoon: false},
  mcp: {Icon: Server, comingSoon: false},
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
  margin: '0.1rem 0 0.5rem var(--ifm-menu-link-padding-horizontal, 0.75rem)',
  padding: '0.65rem 0.75rem',
  textAlign: 'left',
  textDecoration: 'none',
  transition: 'border-color 0.2s ease, box-shadow 0.2s ease',
  width: 'calc(100% - var(--ifm-menu-link-padding-horizontal, 0.75rem))',
  '&:hover': {
    textDecoration: 'none',
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

const chevronSx = {
  color: 'var(--ifm-color-content-secondary)',
  display: 'inline-flex',
  flexShrink: 0,
  transition: 'transform 0.2s ease',
};

const tabListSx = {
  display: 'flex',
  gap: '0.25rem',
  margin: '0 0 0.4rem 1.5rem',
  padding: '0.2rem',
  background: 'rgba(255, 255, 255, 0.05)',
  border: '1px solid rgba(255, 255, 255, 0.12)',
  borderRadius: '8px',
  '[data-theme="light"] &': {
    background: 'rgba(0, 0, 0, 0.03)',
    borderColor: 'rgba(0, 0, 0, 0.1)',
  },
};

const tabSx = {
  background: 'transparent',
  border: 'none',
  borderRadius: '6px',
  color: 'var(--ifm-color-content-secondary)',
  cursor: 'pointer',
  flex: 1,
  fontSize: '0.8rem',
  fontWeight: 600,
  padding: '0.3rem 0.5rem',
  transition: 'background 0.2s ease, color 0.2s ease',
  '&:hover': {color: 'var(--ifm-font-color-base)'},
};

const tabActiveSx = {
  background: 'color-mix(in srgb, var(--ifm-color-primary) 18%, transparent)',
  color: 'var(--ifm-color-primary)',
  '&:hover': {color: 'var(--ifm-color-primary)'},
};

type SidebarCategory = Extract<PropSidebarItem, {type: 'category'}>;

// A connect card whose children are categories marked `connect-app-tab` (B2C and B2B under
// Applications) shows them as horizontal tabs, and lists the quickstarts of the selected tab.
function tabsOf(items: PropSidebarItem[]): SidebarCategory[] {
  return items.filter(
    (i): i is SidebarCategory => i.type === 'category' && !!i.className?.includes('connect-app-tab'),
  );
}

function ConnectSection({item, ...rest}: OriginalProps): React.ReactElement {
  const {items, label, className, href} = item;
  const type = connectTypeFromClassName(className)!;
  const {Icon, comingSoon} = SECTIONS[type];
  const active = useConnectType();
  const expanded = !comingSoon && active === type;

  const tabs = tabsOf(items);
  const activeTabIndex = tabs.findIndex(tab => isActiveSidebarItem(tab, rest.activePath));
  const [selectedTab, setSelectedTab] = useState(Math.max(activeTabIndex, 0));
  // Follow navigation: opening a quickstart selects the tab that holds it, even after the other tab
  // was picked by hand.
  const [lastActivePath, setLastActivePath] = useState(rest.activePath);
  if (rest.activePath !== lastActivePath) {
    setLastActivePath(rest.activePath);
    if (activeTabIndex >= 0) setSelectedTab(activeTabIndex);
  }
  const tabIdPrefix = useId();
  const panelId = `${tabIdPrefix}-panel`;
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  // Arrow keys, Home and End move between the tabs and select the one they land on.
  const onTabKeyDown = (event: React.KeyboardEvent, index: number): void => {
    const last = tabs.length - 1;
    const next = {
      ArrowRight: index === last ? 0 : index + 1,
      ArrowLeft: index === 0 ? last : index - 1,
      Home: 0,
      End: last,
    }[event.key];
    if (next === undefined) return;
    event.preventDefault();
    setSelectedTab(next);
    tabRefs.current[next]?.focus();
  };
  const listItems = tabs.length > 0 ? tabs[selectedTab].items : items;
  const list = (
    <DocSidebarItems
      items={listItems}
      tabIndex={expanded ? 0 : -1}
      activePath={rest.activePath}
      onItemClick={rest.onItemClick}
      level={rest.level + 1}
    />
  );

  return (
    <li className={`menu__list-item connect-section connect-section--${type}`}>
      <Box
        component={comingSoon ? 'div' : href ? Link : 'button'}
        to={comingSoon ? undefined : href}
        type={comingSoon || href ? undefined : 'button'}
        aria-expanded={comingSoon ? undefined : expanded}
        onClick={comingSoon ? undefined : () => {
          applyConnectType(href ? type : expanded ? null : type);
          if (href) rest.onItemClick?.(item);
        }}
        sx={{...cardSx, ...(expanded ? cardActiveSx : {}), ...(comingSoon ? cardDisabledSx : {})}}
      >
        <Box className="cts-opt-icon" component="span" sx={iconBoxSx}>
          <Icon aria-hidden size={20} />
        </Box>
        <Box component="span" sx={{flex: 1}}>{label}</Box>
        {comingSoon ? (
          <Box component="span" sx={badgeSx}>Soon</Box>
        ) : (
          <Box component="span" sx={{...chevronSx, transform: expanded ? 'rotate(180deg)' : 'none'}}>
            <ChevronDown aria-hidden size={14} />
          </Box>
        )}
      </Box>

      {!comingSoon && tabs.length === 0 && (
        <Collapsible lazy as="ul" className="menu__list" collapsed={!expanded}>
          {list}
        </Collapsible>
      )}

      {!comingSoon && tabs.length > 0 && (
        <Collapsible lazy as="div" className="connect-section__panel" collapsed={!expanded}>
          <Box role="tablist" aria-label={label} sx={tabListSx}>
            {tabs.map((tab, index) => (
              <Box
                key={tab.label}
                ref={(element: HTMLButtonElement | null) => {
                  tabRefs.current[index] = element;
                }}
                id={`${tabIdPrefix}-tab-${index}`}
                component="button"
                type="button"
                role="tab"
                aria-selected={index === selectedTab}
                aria-controls={panelId}
                tabIndex={expanded && index === selectedTab ? 0 : -1}
                onClick={() => setSelectedTab(index)}
                onKeyDown={(event: React.KeyboardEvent) => onTabKeyDown(event, index)}
                sx={{...tabSx, ...(index === selectedTab ? tabActiveSx : {})}}
              >
                {tab.label}
              </Box>
            ))}
          </Box>
          <ul className="menu__list" role="tabpanel" id={panelId} aria-labelledby={`${tabIdPrefix}-tab-${selectedTab}`}>
            {list}
          </ul>
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
