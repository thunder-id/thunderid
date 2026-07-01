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

import {Box} from '@wso2/oxygen-ui';
import {Bot, Check, ChevronDown, MonitorSmartphone, Server} from '@wso2/oxygen-ui-icons-react';
import React, {useEffect, useRef, useState} from 'react';

const STORAGE_KEY = 'thunder-connect-type';

const OPTIONS = [
  {Icon: MonitorSmartphone, description: 'Web, mobile & desktop apps', label: 'Application', value: 'app', comingSoon: false},
  {Icon: Bot, description: 'LLM-powered AI agents', label: 'AI Agent', value: 'agent', comingSoon: true},
  {Icon: Server, description: 'Model Context Protocol servers', label: 'MCP Server', value: 'mcp', comingSoon: true},
] as const;

type ConnectType = (typeof OPTIONS)[number]['value'];

const VALID_TYPES = new Set<string>(OPTIONS.map(o => o.value));

function toConnectType(raw: string | null): ConnectType {
  return raw !== null && VALID_TYPES.has(raw) ? (raw as ConnectType) : 'app';
}

function applyType(type: ConnectType): void {
  document.documentElement.dataset.connectType = type;
  localStorage.setItem(STORAGE_KEY, type);
}


const triggerSx = {
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

const triggerOpenSx = {
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 55%, transparent)',
  boxShadow: '0 0 0 3px color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)',
};

const triggerIconSx = {
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

const panelSx = {
  animation: 'connect-panel-enter 0.16s ease',
  background: '#ffffff',
  border: '1px solid var(--ifm-color-emphasis-200)',
  borderRadius: '12px',
  boxShadow: '0 8px 32px rgba(0, 0, 0, 0.1)',
  left: 0,
  marginTop: '0.3rem',
  padding: '0.35rem',
  position: 'absolute',
  right: 0,
  zIndex: 300,
  '[data-theme="dark"] &': {
    background: '#0e1929',
    borderColor: 'rgba(255, 255, 255, 0.08)',
    boxShadow: '0 8px 32px rgba(0, 0, 0, 0.3)',
  },
};

const optionSx = {
  alignItems: 'center',
  background: 'transparent',
  border: 'none',
  borderRadius: '9px',
  color: 'var(--ifm-font-color-base)',
  cursor: 'pointer',
  display: 'flex',
  gap: '0.75rem',
  padding: '0.6rem 0.65rem',
  textAlign: 'left',
  transition: 'background 0.12s ease',
  width: '100%',
  '&:hover': {
    background: 'var(--ifm-color-emphasis-200)',
  },
  '&:hover .cts-opt-icon': {
    background: 'color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)',
    color: 'var(--ifm-color-primary)',
  },
};

const optionActiveSx = {
  background: 'color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)',
  '&:hover': {
    background: 'color-mix(in srgb, var(--ifm-color-primary) 14%, transparent)',
  },
  '& .cts-opt-icon': {
    background: 'color-mix(in srgb, var(--ifm-color-primary) 22%, transparent)',
    color: 'var(--ifm-color-primary)',
  },
  '& .cts-opt-label': {color: 'var(--ifm-color-primary)'},
};

const optionIconSx = {
  alignItems: 'center',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 14%, transparent)',
  borderRadius: '8px',
  color: 'var(--ifm-color-content-secondary)',
  display: 'inline-flex',
  flexShrink: 0,
  height: '2.2rem',
  justifyContent: 'center',
  transition: 'background 0.12s ease, color 0.12s ease',
  width: '2.2rem',
};

export default function ConnectTypeSelector(): React.ReactElement {
  const [selected, setSelected] = useState<ConnectType>(() => {
    if (typeof window !== 'undefined') {
      return toConnectType(localStorage.getItem(STORAGE_KEY));
    }
    return 'app';
  });
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    applyType(selected);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent): void {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  function handleSelect(value: ConnectType): void {
    setSelected(value);
    applyType(value);
    setOpen(false);
  }

  const selectedOption = OPTIONS.find(o => o.value === selected)!;

  return (
    <Box ref={ref} sx={{position: 'relative'}}>
      <Box sx={{
        color: 'var(--ifm-color-content-secondary)',
        fontSize: '0.72rem',
        fontWeight: 500,
        letterSpacing: '0.03em',
        marginTop: '0.75rem',
        padding: '0 var(--ifm-menu-link-padding-horizontal, 0.75rem) 0.3rem',
        textTransform: 'uppercase',
      }}>What are you building?</Box>
      <Box
        aria-expanded={open}
        aria-haspopup="listbox"
        component="button"
        onClick={() => setOpen(v => !v)}
        sx={open ? {...triggerSx, ...triggerOpenSx} : triggerSx}
        type="button"
      >
        <Box component="span" sx={triggerIconSx}>
          <selectedOption.Icon aria-hidden size={20} />
        </Box>
        <Box component="span" sx={{flex: 1}}>{selectedOption.label}</Box>
        <Box
          component="span"
          sx={{
            color: 'var(--ifm-color-content-secondary)',
            display: 'inline-flex',
            flexShrink: 0,
            transform: open ? 'rotate(180deg)' : 'none',
            transition: 'transform 0.2s ease',
          }}
        >
          <ChevronDown aria-hidden size={14} />
        </Box>
      </Box>

      {open && (
        <Box role="listbox" sx={panelSx}>
          {OPTIONS.map(({Icon, description, label, value, comingSoon}) => {
            const isActive = selected === value;
            return (
              <Box
                aria-disabled={comingSoon}
                aria-selected={isActive}
                component="button"
                disabled={comingSoon}
                key={value}
                onClick={comingSoon ? undefined : () => handleSelect(value)}
                role="option"
                sx={{
                  ...(isActive ? {...optionSx, ...optionActiveSx} : optionSx),
                  ...(comingSoon ? {
                    cursor: 'default',
                    opacity: 0.45,
                    pointerEvents: 'none',
                  } : {}),
                }}
                type="button"
              >
                <Box className="cts-opt-icon" component="span" sx={optionIconSx}>
                  <Icon aria-hidden size={18} />
                </Box>
                <Box component="span" sx={{display: 'flex', flex: 1, flexDirection: 'column', gap: '0.1rem', minWidth: 0}}>
                  <Box className="cts-opt-label" component="span" sx={{alignItems: 'center', display: 'flex', fontSize: '0.875rem', fontWeight: 600, gap: '0.5rem', lineHeight: 1.2}}>
                    {label}
                    {comingSoon && (
                      <Box component="span" sx={{
                        background: 'color-mix(in srgb, var(--ifm-color-emphasis-400) 30%, transparent)',
                        borderRadius: '20px',
                        color: 'var(--ifm-color-content-secondary)',
                        fontSize: '0.6rem',
                        fontWeight: 600,
                        letterSpacing: '0.04em',
                        padding: '0.1rem 0.4rem',
                        textTransform: 'uppercase',
                      }}>
                        Coming Soon
                      </Box>
                    )}
                  </Box>
                  <Box component="span" sx={{color: 'var(--ifm-color-content-secondary)', fontSize: '0.72rem', lineHeight: 1.3}}>
                    {description}
                  </Box>
                </Box>
                {isActive && (
                  <Box component="span" sx={{color: 'var(--ifm-color-primary)', display: 'inline-flex', flexShrink: 0}}>
                    <Check aria-hidden size={14} />
                  </Box>
                )}
              </Box>
            );
          })}
        </Box>
      )}
    </Box>
  );
}
