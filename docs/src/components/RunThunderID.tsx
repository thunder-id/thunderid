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

import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import React, {useState} from 'react';
import ClaudeLogo from './icons/ClaudeLogo';
import CliLogo from './icons/CliLogo';
import CodexLogo from './icons/CodexLogo';
import DockerLogo from './icons/DockerLogo';
import SkillsLogo from './icons/SkillsLogo';

type TabId = 'cli' | 'docker' | 'claude' | 'codex' | 'skills';

interface TabContent {
  command: string;
  hint: string;
  shell: boolean;
}

const TABS: {id: TabId; label: string; icon: React.ReactElement}[] = [
  {id: 'cli',    label: 'CLI',    icon: <CliLogo size={18} />},
  {id: 'docker', label: 'Docker', icon: <DockerLogo size={18} />},
  {id: 'claude', label: 'Claude', icon: <ClaudeLogo size={18} />},
  {id: 'codex',  label: 'Codex',  icon: <CodexLogo size={18} />},
  {id: 'skills', label: 'Skills', icon: <SkillsLogo size={18} />},
];

const CONTENT: Record<TabId, TabContent> = {
  cli:    {command: 'npx thunderid',                                          hint: 'Requires Node.js 18+',               shell: true},
  docker: {command: 'docker compose -f oci://ghcr.io/thunder-id/thunderid-quick-start:latest up', hint: 'Requires Docker and Docker Compose', shell: true},
  claude: {command: '/plugin marketplace add thunder-id/skills',              hint: 'Run in Claude chat',                 shell: true},
  codex:  {command: 'codex plugin marketplace add thunder-id/skills',         hint: 'Run in terminal with Codex installed', shell: false},
  skills: {command: 'npx skills add thunder-id/skills',                       hint: 'Requires Node.js 18+',               shell: true},
};

function CopyButton({text}: {text: string}): React.ReactElement {
  const [copied, setCopied] = useState(false);
  const handleCopy = (): void => {
    void navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    });
  };
  return (
    <Box
      component="button"
      onClick={handleCopy}
      sx={{
        alignItems: 'center',
        background: 'rgba(255,255,255,0.08)',
        border: '1px solid rgba(255,255,255,0.14)',
        borderRadius: '7px',
        color: copied ? 'success.main' : 'text.secondary',
        cursor: 'pointer',
        display: 'flex',
        flexShrink: 0,
        fontSize: '0.73rem',
        fontWeight: 600,
        letterSpacing: '0.03em',
        px: 1.75,
        py: 0.6,
        transition: 'color 0.15s, background 0.15s',
        '[data-theme="light"] &': {background: 'rgba(0,0,0,0.05)', border: '1px solid rgba(0,0,0,0.1)'},
        '&:hover': {background: 'rgba(255,255,255,0.14)', color: 'text.primary',
          '[data-theme="light"] &': {background: 'rgba(0,0,0,0.08)'},
        },
      }}
    >
      {copied ? 'Copied!' : 'Copy'}
    </Box>
  );
}

interface RunThunderIDProps {
  tabs?: TabId[];
  defaultTab?: TabId;
}

export default function RunThunderID({tabs, defaultTab}: RunThunderIDProps = {}): React.ReactElement {
  const visibleTabs = tabs ? TABS.filter(({id}) => tabs.includes(id)) : TABS;
  const [activeTab, setActiveTab] = useState<TabId>(defaultTab ?? visibleTabs[0]?.id ?? 'cli');
  const theme = useTheme();
  const {command, hint, shell} = CONTENT[activeTab];

  return (
    <Box
      sx={{
        borderRadius: '14px',
        border: '1px solid',
        borderColor: 'divider',
        overflow: 'hidden',
        my: 2,
        '@keyframes cmdFadeIn': {
          from: {opacity: 0, transform: 'translateY(5px)'},
          to:   {opacity: 1, transform: 'translateY(0)'},
        },
      }}
    >
      {/* Method selector */}
      <Box
        sx={{
          borderBottom: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          gap: 3,
          px: 2,
          pt: 1.75,
          pb: 1.5,
          bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.03)`,
          '[data-theme="light"] &': {bgcolor: 'rgba(0,0,0,0.02)'},
        }}
      >
        {visibleTabs.map(({id, label, icon}) => {
          const isActive = activeTab === id;
          return (
            <Box
              key={id}
              component="button"
              onClick={() => setActiveTab(id)}
              sx={{
                background: isActive
                  ? `rgba(${theme.vars?.palette.primary.main} / 0.14)`
                  : 'transparent',
                border: 'none',
                borderRadius: '10px',
                boxShadow: isActive
                  ? `inset 0 0 0 1.5px rgba(${theme.vars?.palette.primary.main} / 0.4)`
                  : 'none',
                color: isActive ? 'primary.main' : 'text.disabled',
                cursor: 'pointer',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 0.75,
                px: 1.5,
                py: 1,
                transition: 'background 0.18s, color 0.18s, box-shadow 0.18s',
                '& .method-icon': {
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  filter: isActive ? 'none' : 'grayscale(1) opacity(0.55)',
                  transition: 'filter 0.18s',
                },
                '&:hover': {
                  color: isActive ? 'primary.main' : 'text.secondary',
                  background: isActive
                    ? `rgba(${theme.vars?.palette.primary.main} / 0.14)`
                    : `rgba(${theme.vars?.palette.primary.main} / 0.06)`,
                  '& .method-icon': {filter: 'grayscale(0.2) opacity(0.9)'},
                },
              }}
            >
              <Box className="method-icon">{icon}</Box>
              <Typography
                component="span"
                sx={{
                  fontSize: '0.7rem',
                  fontWeight: isActive ? 600 : 400,
                  letterSpacing: '0.01em',
                  lineHeight: 1,
                  textAlign: 'center',
                }}
              >
                {label}
              </Typography>
            </Box>
          );
        })}
      </Box>

      {/* Command — fades in on tab switch */}
      <Box
        key={activeTab}
        sx={{
          alignItems: 'center',
          animation: 'cmdFadeIn 0.2s ease',
          bgcolor: 'rgba(0,0,0,0.18)',
          display: 'flex',
          gap: 2,
          justifyContent: 'space-between',
          px: 2.5,
          py: 2,
          '[data-theme="light"] &': {bgcolor: 'rgba(0,0,0,0.03)'},
        }}
      >
        <Box sx={{display: 'flex', alignItems: 'baseline', gap: 1.5, minWidth: 0}}>
          {shell && (
            <Typography
              component="span"
              sx={{color: 'text.disabled', fontFamily: 'monospace', fontSize: '0.85rem', flexShrink: 0, userSelect: 'none'}}
            >
              $
            </Typography>
          )}
          <Typography
            component="span"
            sx={{
              color: 'text.primary',
              fontFamily: 'monospace',
              fontSize: '0.95rem',
              fontWeight: 500,
              lineHeight: 1.5,
              overflow: 'hidden',
              textOverflow: shell ? 'ellipsis' : 'unset',
              whiteSpace: shell ? 'nowrap' : 'normal',
            }}
          >
            {command}
          </Typography>
        </Box>
        <CopyButton text={command} />
      </Box>

      {/* Footer */}
      <Box
        sx={{
          alignItems: 'center',
          borderTop: '1px solid',
          borderColor: 'divider',
          display: 'flex',
          justifyContent: 'space-between',
          px: 2.5,
          py: 0.85,
        }}
      >
        <Typography sx={{color: 'text.disabled', fontSize: '0.75rem'}}>{hint}</Typography>
        <Box
          component="a"
          href="/docs/next/guides/getting-started/get-thunderid"
          sx={{
            color: 'text.disabled',
            flexShrink: 0,
            fontSize: '0.75rem',
            ml: 2,
            textDecoration: 'none',
            whiteSpace: 'nowrap',
            transition: 'color 0.15s',
            '&:hover': {color: 'primary.main'},
          }}
        >
          Full install guide →
        </Box>
      </Box>
    </Box>
  );
}
