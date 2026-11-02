// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Check, FingerprintPattern, KeyRound, MailCheck} from '@wso2/oxygen-ui-icons-react';
import React from 'react';
import {useUrlSelection} from '@site/src/utils/useUrlSelection';

// Which credential the agent signs in with, shared by the method picker and
// every <SignInMethodContent> block. It lives in the URL query string
// (?credential=...) rather than a store, so the choice is shareable via the
// link and in sync across blocks, and a fresh visit with no query starts on
// "password".
type SignInMethod = 'password' | 'otp' | 'passkey';

const DEFAULT_METHOD: SignInMethod = 'password';

const METHODS: {value: SignInMethod; label: string; description: string; Icon: typeof KeyRound}[] = [
  {
    value: 'password',
    label: 'Username and Password',
    description: "Credentials an agent knows, similar to a person's account.",
    Icon: KeyRound,
  },
  {
    value: 'otp',
    label: 'Email or SMS OTP',
    description: 'A one-time code or magic link the agent reads from its own inbox or SMS gateway.',
    Icon: MailCheck,
  },
  {
    value: 'passkey',
    label: 'Passkey',
    description: 'A WebAuthn credential the agent registers and signs with, using a key pair instead of a shared secret.',
    Icon: FingerprintPattern,
  },
];

const METHOD_VALUES = METHODS.map(m => m.value) as readonly SignInMethod[];

/**
 * One method's content. Every block stays in the DOM and inactive ones are
 * hidden, so both credentials remain searchable rather than existing only
 * after a click.
 */
export function SignInMethodContent({
  value,
  children = null,
}: {
  value: SignInMethod;
  children?: React.ReactNode;
}): React.ReactElement {
  const [active] = useUrlSelection('credential', METHOD_VALUES, DEFAULT_METHOD);
  return <Box hidden={value !== active}>{children}</Box>;
}

export function SignInMethodSelector(): React.ReactElement {
  const [active, setMethod] = useUrlSelection('credential', METHOD_VALUES, DEFAULT_METHOD);

  return (
    <Box
      role="radiogroup"
      aria-label="How the agent signs in"
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
