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

import {Box, Typography} from '@wso2/oxygen-ui';
import {Bot, Check, UserCheck} from '@wso2/oxygen-ui-icons-react';
import React from 'react';
import {useUrlSelection} from '@site/src/utils/useUrlSelection';

// Which identity the agent runs under, shared by the mode picker and every
// <Mode> block on the page. It lives in the URL query string (?mode=...) rather
// than a store, so the choice is shareable via the link and in sync across
// blocks, and a fresh visit with no query starts on "own".
type AgentMode = 'own' | 'obo';

const DEFAULT_MODE: AgentMode = 'own';

const MODES: {value: AgentMode; label: string; description: string; Icon: typeof Bot}[] = [
  {
    value: 'own',
    label: 'Acting on Its Own',
    description:
      "The agent gets its own identity and authenticates with its own credentials, acting on its own behalf with no user in the loop.",
    Icon: Bot,
  },
  {
    value: 'obo',
    label: 'On Behalf of a User',
    description:
      "The user signs in once and consents, and the agent acts as that user, with the user's delegated authority and never exceeding it.",
    Icon: UserCheck,
  },
];

const MODE_VALUES = MODES.map(m => m.value) as readonly AgentMode[];

/**
 * One mode's content. Every block stays in the DOM and inactive ones are
 * hidden, so both scenarios remain searchable rather than existing only
 * after a click.
 */
export function Mode({value, children = null}: {value: AgentMode; children?: React.ReactNode}): React.ReactElement {
  const [active] = useUrlSelection('mode', MODE_VALUES, DEFAULT_MODE);
  return <Box hidden={value !== active}>{children}</Box>;
}

export function AgentModeSelector(): React.ReactElement {
  const [active, setMode] = useUrlSelection('mode', MODE_VALUES, DEFAULT_MODE);

  return (
    <Box
      role="radiogroup"
      aria-label="How the agent behaves"
      sx={{display: 'grid', gap: 1.5, gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr'}, my: 2}}
    >
      {MODES.map(({value, label, description, Icon}) => {
        const isActive = active === value;
        return (
          <Box
            key={value}
            component="button"
            type="button"
            role="radio"
            aria-checked={isActive}
            onClick={() => setMode(value)}
            sx={{
              background: isActive
                ? 'color-mix(in srgb, var(--ifm-color-primary) 8%, transparent)'
                : 'var(--ifm-background-surface-color)',
              border: '1.5px solid',
              borderColor: isActive
                ? 'color-mix(in srgb, var(--ifm-color-primary) 55%, transparent)'
                : 'var(--ifm-color-emphasis-300)',
              borderRadius: '12px',
              cursor: 'pointer',
              display: 'flex',
              flexDirection: 'column',
              gap: '0.5rem',
              padding: '1rem',
              textAlign: 'left',
              transition: 'border-color 0.15s ease, background 0.15s ease',
              width: '100%',
              '&:hover': {
                borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 45%, transparent)',
              },
            }}
          >
            <Box sx={{alignItems: 'center', display: 'flex', gap: '0.5rem'}}>
              <Box
                aria-hidden
                sx={{
                  alignItems: 'center',
                  color: isActive ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-700)',
                  display: 'inline-flex',
                }}
              >
                <Icon size={18} />
              </Box>
              <Typography
                component="span"
                sx={{
                  color: isActive ? 'var(--ifm-color-primary)' : 'var(--ifm-font-color-base)',
                  flex: 1,
                  fontSize: '0.95rem',
                  fontWeight: 700,
                }}
              >
                {label}
              </Typography>
              {isActive && (
                <Box
                  aria-hidden
                  sx={{
                    alignItems: 'center',
                    background: 'var(--ifm-color-primary)',
                    borderRadius: '50%',
                    color: '#fff',
                    display: 'inline-flex',
                    height: 18,
                    justifyContent: 'center',
                    width: 18,
                  }}
                >
                  <Check size={12} />
                </Box>
              )}
            </Box>
            <Typography
              component="span"
              sx={{color: 'var(--ifm-color-content-secondary)', fontSize: '0.82rem', lineHeight: 1.5}}
            >
              {description}
            </Typography>
          </Box>
        );
      })}
    </Box>
  );
}
