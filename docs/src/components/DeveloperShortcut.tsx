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
import {useWindowSize} from '@docusaurus/theme-common';
import {Box, Chip, Typography} from '@wso2/oxygen-ui';
import {Bot, Check, Download, MonitorSmartphone, Server, Zap} from '@wso2/oxygen-ui-icons-react';
import React, {useCallback, useState} from 'react';
import AndroidLogo from './icons/AndroidLogo';
import ExpressLogo from './icons/ExpressLogo';
import FlutterLogo from './icons/FlutterLogo';
import IOSLogo from './icons/IOSLogo';
import JavaScriptLogo from './icons/JavaScriptLogo';
import NextLogo from './icons/NextLogo';
import NodeLogo from './icons/NodeLogo';
import NuxtLogo from './icons/NuxtLogo';
import ReactLogo from './icons/ReactLogo';
import VueLogo from './icons/VueLogo';
import {CONNECT_TYPE_STORAGE_KEY, applyConnectType, toConnectType} from '../utils/connectType';

type ConnectType = 'app' | 'agent' | 'mcp';

const ALL_FRAMEWORKS = [
  {Logo: ReactLogo,      href: '/docs/next/getting-started/connect-your-application/react',   label: 'React'},
  {Logo: NextLogo,       href: '/docs/next/getting-started/connect-your-application/nextjs',  label: 'Next.js'},
  {Logo: ExpressLogo,    href: '/docs/next/getting-started/connect-your-application/express', label: 'Express'},
  {Logo: VueLogo,        href: '/docs/next/getting-started/connect-your-application/vue',     label: 'Vue'},
  {Logo: NuxtLogo,       href: '/docs/next/getting-started/connect-your-application/nuxt',    label: 'Nuxt'},
  {Logo: NodeLogo,       href: '/docs/next/getting-started/connect-your-application/node',    label: 'Node.js'},
  {Logo: JavaScriptLogo, href: '/docs/next/getting-started/connect-your-application/browser', label: 'JavaScript'},
  {Logo: IOSLogo,        href: '/docs/next/getting-started/connect-your-application/ios',     label: 'iOS'},
  {Logo: AndroidLogo,    href: '/docs/next/getting-started/connect-your-application/android', label: 'Android'},
  {Logo: FlutterLogo,    href: '/docs/next/getting-started/connect-your-application/flutter', label: 'Flutter'},
];

const CATEGORIES: {id: ConnectType; icon: React.ReactElement; label: string; description: string; comingSoon: boolean}[] = [
  {id: 'app',   icon: <MonitorSmartphone size={20} />, label: 'Application', description: 'Web, mobile and desktop apps.', comingSoon: false},
  {id: 'agent', icon: <Bot size={20} />,               label: 'AI Agent',    description: 'LLM-powered agents.',            comingSoon: true},
  {id: 'mcp',   icon: <Server size={20} />,            label: 'MCP Server',  description: 'Model Context Protocol servers.', comingSoon: true},
];

function selectCategory(type: ConnectType): void {
  applyConnectType(type);
  const items = document.querySelectorAll<HTMLElement>(`.connect-section--${type} > .menu__list > li`);
  items.forEach(el => {
    el.style.animation = 'none';
    void el.offsetHeight;
    el.style.animation = '';
  });
}

function openMobileSidebar(type: ConnectType): void {
  selectCategory(type);
  const btn = (
    document.querySelector<HTMLButtonElement>('[aria-label="Toggle navigation bar"]') ??
    document.querySelector<HTMLButtonElement>('[aria-label="Toggle sidebar"]') ??
    document.querySelector<HTMLButtonElement>('button[class*="sidebarToggle"], button[class*="sidebar_"]')
  );
  btn?.click();
}

interface DeveloperShortcutProps {
  title?: string;
  subtitle?: string;
  showInstallPath?: boolean;
  compact?: boolean;
}

export default function DeveloperShortcut({
  title = 'Choose a quickstart path.',
  subtitle = 'Select what you\'re building, then start with a quickstart for your framework or platform.',
  showInstallPath = false,
  compact = false,
}: DeveloperShortcutProps): React.ReactElement {
  const windowSize = useWindowSize();
  const isMobile = windowSize === 'mobile';

  const [selected, setSelected] = useState<ConnectType>(() => {
    if (typeof window !== 'undefined') {
      return toConnectType(localStorage.getItem(CONNECT_TYPE_STORAGE_KEY));
    }
    return 'app';
  });

  const handleSelect = useCallback((type: ConnectType, comingSoon: boolean) => {
    if (comingSoon) return;
    setSelected(type);
    if (isMobile) {
      openMobileSidebar(type);
    } else {
      selectCategory(type);
    }
  }, [isMobile]);

  // ─── Compact mode (used on Get ThunderID page) ───────────────────────────
  if (compact) {
    return (
      <Box
        sx={{
          borderRadius: '14px',
          border: '1px solid rgba(255,255,255,0.1)',
          bgcolor: 'rgba(255,255,255,0.05)',
          my: 2,
          p: 2.5,
          '[data-theme="light"] &': {border: '1px solid #c2d7f5', bgcolor: '#e8f1fc'},
        }}
      >
        {/* Title + subtitle */}
        <Box sx={{display: 'flex', gap: 1.25, mb: 0}}>
          <Box
            sx={{
              alignItems: 'center',
              bgcolor: 'color-mix(in srgb, var(--ifm-color-primary) 15%, transparent)',
              borderRadius: '6px',
              color: 'primary.main',
              display: 'flex',
              flexShrink: 0,
              height: 24,
              justifyContent: 'center',
              mt: 0.1,
              width: 24,
            }}
          >
            <Zap size={13} />
          </Box>
          <Box>
            <Typography sx={{fontWeight: 700, fontSize: '0.85rem', color: 'text.primary', lineHeight: 1.3}}>
              {title}
            </Typography>
            {subtitle && (
              <Typography sx={{fontSize: '0.78rem', color: 'text.secondary', mt: 0.3, lineHeight: 1.5}}>
                {subtitle}
              </Typography>
            )}
          </Box>
        </Box>

      </Box>
    );
  }

  // ─── Full mode (used on docs home page) ──────────────────────────────────
  const selectedCategory = CATEGORIES.find(c => c.id === selected)!;

  return (
    <Box
      sx={{
        borderRadius: '16px',
        border: '1px solid rgba(255,255,255,0.1)',
        bgcolor: 'rgba(255,255,255,0.05)',
        boxShadow: '0 8px 32px rgba(0,0,0,0.3)',
        overflow: 'hidden',
        my: 3,
        '[data-theme="light"] &': {
          border: '1px solid #c2d7f5',
          bgcolor: '#e8f1fc',
          boxShadow: '0 4px 24px rgba(30, 100, 200, 0.1)',
        },
      }}
    >
      {/* Header */}
      <Box sx={{px: {xs: 2.5, md: 3.5}, pt: 3, pb: 0}}>
        <Typography sx={{color: 'text.primary', fontSize: {xs: '1rem', sm: '1.1rem', md: '1.2rem'}, fontWeight: 800, letterSpacing: '-0.01em', mb: 0.4}}>
          {title}
        </Typography>
        <Typography sx={{color: 'text.secondary', fontSize: '0.875rem', mb: 2.5}}>
          {subtitle}
        </Typography>
      </Box>

      {/* Category selector */}
      <Box sx={{px: {xs: 2.5, md: 3.5}, pb: 2.5}}>
        <Typography sx={{color: 'text.disabled', fontSize: '0.68rem', fontWeight: 700, letterSpacing: '0.1em', mb: 1, textTransform: 'uppercase'}}>
          What are you building?
        </Typography>
        <Box sx={{display: 'grid', gap: 1.25, gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))'}}>
          {CATEGORIES.map(({id, icon, label, description, comingSoon}) => {
            const isSelected = selected === id && !comingSoon;
            return (
              <Box
                key={id}
                component={comingSoon ? 'div' : 'button'}
                onClick={comingSoon ? undefined : () => handleSelect(id, comingSoon)}
                sx={{
                  background: 'none',
                  border: '1.5px solid',
                  borderColor: isSelected
                    ? 'color-mix(in srgb, var(--ifm-color-primary) 45%, transparent)'
                    : 'rgba(255,255,255,0.08)',
                  borderRadius: '12px',
                  bgcolor: isSelected
                    ? 'color-mix(in srgb, var(--ifm-color-primary) 8%, transparent)'
                    : 'rgba(255,255,255,0.03)',
                  cursor: comingSoon ? 'default' : 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 0.75,
                  opacity: comingSoon ? 0.45 : 1,
                  p: 2,
                  textAlign: 'left',
                  transition: 'border-color 0.15s, background-color 0.15s',
                  width: '100%',
                  '[data-theme="light"] &': {
                    borderColor: isSelected ? '#93c5fd' : 'rgba(0,0,0,0.08)',
                    bgcolor: isSelected ? '#eff6ff' : '#ffffff',
                  },
                  ...(!comingSoon && !isSelected && {
                    '&:hover': {
                      borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 40%, transparent)',
                      bgcolor: 'color-mix(in srgb, var(--ifm-color-primary) 7%, transparent)',
                    },
                  }),
                }}
              >
                <Box sx={{alignItems: 'center', display: 'flex', justifyContent: 'space-between'}}>
                  <Box sx={{alignItems: 'center', color: isSelected ? 'primary.main' : 'text.secondary', display: 'flex', gap: 0.75}}>
                    {icon}
                    <Typography sx={{color: isSelected ? 'primary.main' : 'text.primary', fontSize: '0.9rem', fontWeight: 700}}>
                      {label}
                    </Typography>
                  </Box>
                  {isSelected && (
                    <Box sx={{alignItems: 'center', bgcolor: 'primary.main', borderRadius: '50%', color: '#fff', display: 'flex', height: 20, justifyContent: 'center', width: 20}}>
                      <Check size={12} />
                    </Box>
                  )}
                  {comingSoon && <Chip label="Soon" size="small" sx={{fontSize: '0.65rem', height: 18}} />}
                </Box>
                <Typography sx={{color: 'text.secondary', fontSize: '0.78rem', lineHeight: 1.5}}>
                  {description}
                </Typography>
              </Box>
            );
          })}
        </Box>
      </Box>

      {/* Quickstart links */}
      <Box sx={{px: {xs: 2.5, md: 3.5}, pb: 3}}>

        {selected === 'app' ? (
          <Box>
            <Typography sx={{color: 'text.disabled', fontSize: '0.68rem', fontWeight: 700, letterSpacing: '0.08em', mb: 1, textTransform: 'uppercase'}}>
              Popular quickstarts
            </Typography>
            <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 0.875, mb: 2}}>
              {ALL_FRAMEWORKS.filter(f => ['React','Next.js','Express','Vue'].includes(f.label)).map(({Logo, href, label}) => (
                <Box
                  key={label}
                  component={Link}
                  to={href}
                  sx={{
                    alignItems: 'center',
                    bgcolor: 'rgba(255,255,255,0.06)',
                    border: '1px solid rgba(255,255,255,0.1)',
                    borderRadius: '8px',
                    color: 'text.primary',
                    display: 'flex',
                    fontSize: '0.82rem',
                    fontWeight: 500,
                    gap: 0.75,
                    px: 1.5,
                    py: 0.75,
                    textDecoration: 'none !important',
                    transition: 'border-color 0.15s, color 0.15s',
                    '[data-theme="light"] &': {bgcolor: '#ffffff', border: '1px solid rgba(0,0,0,0.1)'},
                    '&:hover': {
                      borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 50%, transparent)',
                      color: 'primary.main',
                      textDecoration: 'none !important',
                    },
                  }}
                >
                  <Box sx={{display: 'flex', alignItems: 'center', opacity: 0.9}}>
                    <Logo size={16} />
                  </Box>
                  {label} →
                </Box>
              ))}
            </Box>
            <Typography sx={{color: 'text.secondary', fontSize: '0.8rem'}}>
              All application quickstarts are available in the sidebar.
            </Typography>
          </Box>
        ) : (
          <Typography sx={{color: 'text.disabled', fontSize: '0.85rem'}}>
            {selectedCategory.label} quickstarts are coming soon.
          </Typography>
        )}
      </Box>

      {/* Install path */}
      {showInstallPath && (
        <Box sx={{borderTop: '1px solid', borderColor: 'rgba(255,255,255,0.06)', px: {xs: 2.5, md: 3.5}, py: 1.5, '[data-theme="light"] &': {borderColor: 'rgba(0,0,0,0.07)'}}}>
          <Box sx={{alignItems: {xs: 'flex-start', sm: 'center'}, display: 'flex', flexDirection: {xs: 'column', sm: 'row'}, gap: {xs: 1, sm: 2}, justifyContent: 'space-between'}}>
            <Box sx={{alignItems: 'center', display: 'flex', gap: 1.25, flexWrap: 'wrap'}}>
              <Box
                sx={{
                  alignItems: 'center',
                  bgcolor: 'rgba(255,255,255,0.07)',
                  borderRadius: '7px',
                  color: 'text.secondary',
                  display: 'flex',
                  height: 28,
                  justifyContent: 'center',
                  width: 28,
                  flexShrink: 0,
                  '[data-theme="light"] &': {bgcolor: 'rgba(0,0,0,0.06)'},
                }}
              >
                <Download size={14} />
              </Box>
              <Typography sx={{color: 'text.secondary', fontSize: '0.875rem'}}>
                Just want ThunderID running?
              </Typography>
              <Box
                component={Link}
                to="/docs/next/getting-started/get-thunderid"
                sx={{
                  color: 'primary.main',
                  fontSize: '0.875rem',
                  fontWeight: 600,
                  textDecoration: 'none',
                  '&:hover': {textDecoration: 'underline'},
                }}
              >
                Get ThunderID →
              </Box>
            </Box>
          </Box>
        </Box>
      )}
    </Box>
  );
}
