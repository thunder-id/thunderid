// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Switch, Typography, alpha, keyframes, useTheme} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

const pop = keyframes`
  0% { transform: scale(1); }
  45% { transform: scale(1.1); }
  100% { transform: scale(1); }
`;

/**
 * Props for the AuthenticationMethodItem component
 */
export interface AuthenticationMethodItemProps {
  /**
   * Unique identifier for the authentication method
   */
  id: string;

  /**
   * Display name for the authentication method
   */
  name: string;

  /**
   * Icon for the authentication method
   */
  icon: ReactNode;

  /**
   * Whether this method is currently enabled
   */
  isEnabled: boolean;

  /**
   * Whether this method is available (affects disabled state)
   */
  isAvailable: boolean;

  /**
   * Whether this method should be disabled
   */
  isDisabled?: boolean;

  /**
   * Callback when the method is toggled
   */
  onToggle: (id: string) => void;
}

/**
 * A single authentication method row: an icon avatar, a label, and a trailing control. Only the
 * trailing control (the switch) toggles the method — the icon/label are informational, not a
 * click target, so the row doesn't show button hover/ripple feedback where there's nothing to
 * click. They're placed in a plain flex row (rather than MUI's `secondaryAction`, which
 * absolute-positions the trailing control and can drift out of vertical center once the label
 * wraps to two lines) so both stay centered no matter how much text a row has. When the method
 * isn't available (no connection configured for it yet), the trailing control is a "Not
 * configured" label instead of a switch, since there's nothing to toggle.
 */
export default function AuthenticationMethodItem({
  id,
  name,
  icon,
  isEnabled,
  isAvailable,
  isDisabled = false,
  onToggle,
}: AuthenticationMethodItemProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const disabled = isDisabled || !isAvailable;

  const handleToggle = (): void => {
    if (disabled) return;
    onToggle(id);
  };

  const flash = keyframes`
    0% { background-color: transparent; }
    30% { background-color: ${alpha(theme.palette.primary.main, 0.08)}; }
    100% { background-color: transparent; }
  `;

  return (
    <Box
      data-testid={`auth-method-${id}`}
      role="listitem"
      sx={{display: 'flex', alignItems: 'center', gap: 1.5, width: '100%', px: 2, py: 1.25, position: 'relative'}}
    >
      <Box
        key={isEnabled ? 'enabled' : 'disabled'}
        aria-hidden
        sx={{
          position: 'absolute',
          inset: 0,
          pointerEvents: 'none',
          animation: `${flash} ${theme.transitions.duration.standard}ms ${theme.transitions.easing.easeInOut}`,
        }}
      />

      <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, flex: 1, minWidth: 0}}>
        <Box
          key={isEnabled ? 'enabled' : 'disabled'}
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 36,
            height: 36,
            flexShrink: 0,
            borderRadius: '50%',
            bgcolor: isEnabled ? alpha(theme.palette.primary.main, 0.14) : 'action.hover',
            color: isEnabled ? 'primary.main' : 'text.secondary',
            opacity: disabled ? 0.6 : 1,
            animation: `${pop} ${theme.transitions.duration.short}ms ${theme.transitions.easing.easeInOut}`,
            transition: theme.transitions.create(['background-color', 'color'], {
              duration: theme.transitions.duration.short,
            }),
          }}
        >
          {icon}
        </Box>
        <Box sx={{minWidth: 0}}>
          <Typography variant="body2" fontWeight={600} sx={{opacity: disabled ? 0.6 : 1}}>
            {name}
          </Typography>
        </Box>
      </Box>

      {isAvailable ? (
        <Switch checked={isEnabled} disabled={disabled} onChange={handleToggle} color="primary" />
      ) : (
        <Typography variant="body2" color="text.secondary" fontWeight={600} sx={{whiteSpace: 'nowrap'}}>
          {t('applications:onboarding.configure.SignInOptions.notConfigured')}
        </Typography>
      )}
    </Box>
  );
}
