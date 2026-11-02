// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack} from '@wso2/oxygen-ui';
import {ChevronRight, Rocket} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

export interface PathConnectorProps {
  /**
   * `deploy` joins the versions to a stage: a rocket stands in the middle and the versions' own
   * buttons act. `promote` joins one stage to the next with the button that promotes.
   */
  variant: 'deploy' | 'promote';
  /** The promote button's label; ignored for a deploy connector. */
  label?: string;
  onClick?: () => void;
  disabled?: boolean;
  testId?: string;
}

const Line = (): JSX.Element => <Box sx={{width: 18, height: 1, bgcolor: 'divider'}} />;

/** The dot, lines and arrow between two columns of the deploy path, with what sits in the middle. */
export default function PathConnector({
  variant,
  label = undefined,
  onClick = undefined,
  disabled = false,
  testId = undefined,
}: PathConnectorProps): JSX.Element {
  return (
    <Stack direction="row" alignItems="center" spacing={0.5} sx={{alignSelf: 'center', px: 1}} data-testid={testId}>
      <Box sx={{width: 7, height: 7, borderRadius: '50%', bgcolor: 'text.disabled'}} />
      <Line />
      {variant === 'deploy' ? (
        <Box sx={{display: 'flex', color: 'text.disabled'}} aria-hidden>
          <Rocket size={16} />
        </Box>
      ) : (
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
      <Line />
      <Box sx={{display: 'flex', color: 'text.disabled'}} aria-hidden>
        <ChevronRight size={16} />
      </Box>
    </Stack>
  );
}
