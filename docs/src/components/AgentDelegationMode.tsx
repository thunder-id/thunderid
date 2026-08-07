// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowRightLeft, Check, LogIn, Smartphone} from '@wso2/oxygen-ui-icons-react';
import React from 'react';
import {useUrlSelection} from '@site/src/utils/useUrlSelection';

// How an agent obtains a token that represents a user, shared by the picker and
// every <DelegationContent> block. It lives in the URL query string (?method=...)
// so the choice is shareable and stays in sync across blocks; a fresh visit
// starts on "signin" (the agent signs the user in).
type DelegationMethod = 'signin' | 'approval' | 'exchange';

const DEFAULT_METHOD: DelegationMethod = 'signin';

const METHODS: {value: DelegationMethod; label: string; description: string; Icon: typeof LogIn}[] = [
  {
    value: 'signin',
    label: 'Bring the user in',
    description:
      'No user token yet: the agent signs the user in and receives one, using the authorization code grant.',
    Icon: LogIn,
  },
  {
    value: 'approval',
    label: 'Reach the user',
    description:
      "The user is not present: the agent requests approval on the user's own device over a back channel, using the CIBA grant.",
    Icon: Smartphone,
  },
  {
    value: 'exchange',
    label: 'Trade a user token',
    description:
      'A user token already exists: exchange it for a new one scoped to a downstream API, using the token exchange grant.',
    Icon: ArrowRightLeft,
  },
];

const METHOD_VALUES = METHODS.map(m => m.value) as readonly DelegationMethod[];

/**
 * One method's content. Every block stays in the DOM and inactive ones are
 * hidden, so both paths remain searchable rather than existing only after a click.
 */
export function DelegationContent({
  value,
  children = null,
}: {
  value: DelegationMethod;
  children?: React.ReactNode;
}): React.ReactElement {
  const [active] = useUrlSelection('method', METHOD_VALUES, DEFAULT_METHOD);
  return <Box hidden={value !== active}>{children}</Box>;
}

export function DelegationMethodSelector(): React.ReactElement {
  const [active, setMethod] = useUrlSelection('method', METHOD_VALUES, DEFAULT_METHOD);

  return (
    <Box
      role="radiogroup"
      aria-label="How the agent obtains the user's token"
      sx={{display: 'grid', gap: 1.5, gridTemplateColumns: {xs: '1fr', sm: 'repeat(3, 1fr)'}, my: 2}}
    >
      {METHODS.map(({value, label, description, Icon}) => {
        const isActive = active === value;
        return (
          <Box
            key={value}
            component="button"
            type="button"
            role="radio"
            aria-checked={isActive}
            onClick={() => setMethod(value)}
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
