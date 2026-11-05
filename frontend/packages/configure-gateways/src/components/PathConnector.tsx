// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack} from '@wso2/oxygen-ui';
import {Rocket, Send} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

export interface PathConnectorProps {
  /**
   * `deploy` joins the versions to a stage: a rocket rides the arrow and the versions' own buttons
   * act. `promote` joins one stage to the next: a paper plane rides the arrow, above the button that
   * promotes.
   */
  variant: 'deploy' | 'promote';
  /** The promote button's label; ignored for a deploy connector. */
  label?: string;
  onClick?: () => void;
  disabled?: boolean;
  testId?: string;
}

const STROKE = 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.55)';

const Shaft = (): JSX.Element => (
  <Box aria-hidden sx={{flex: 1, minWidth: 24, height: 2, borderRadius: 1, bgcolor: STROKE}} />
);

/**
 * The arrow between two columns of the deploy path, so the way configuration moves reads at a glance,
 * with what it carries riding on its shaft.
 */
export default function PathConnector({
  variant,
  label = undefined,
  onClick = undefined,
  disabled = false,
  testId = undefined,
}: PathConnectorProps): JSX.Element {
  return (
    <Stack alignItems="center" spacing={1} sx={{alignSelf: 'center', minWidth: 112, mx: 1}} data-testid={testId}>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 1, width: '100%'}}>
        <Shaft />
        <Box
          sx={{display: 'flex', color: 'primary.main'}}
          aria-hidden
          data-testid={variant === 'deploy' ? 'path-rocket' : 'path-plane'}
        >
          {variant === 'deploy' ? <Rocket size={16} /> : <Send size={16} />}
        </Box>
        <Box sx={{flex: 1, minWidth: 24, display: 'flex', alignItems: 'center'}}>
          <Shaft />
          <Box
            aria-hidden
            component="svg"
            viewBox="0 0 10 12"
            sx={{width: 10, height: 12, flexShrink: 0, ml: '-1px', display: 'block'}}
          >
            <path d="M0 0 L10 6 L0 12 Z" fill={STROKE} />
          </Box>
        </Box>
      </Box>
      {variant === 'promote' && (
        <Button
          variant="outlined"
          size="small"
          onClick={onClick}
          disabled={disabled}
          sx={{borderRadius: 100, whiteSpace: 'nowrap'}}
        >
          {label}
        </Button>
      )}
    </Stack>
  );
}
