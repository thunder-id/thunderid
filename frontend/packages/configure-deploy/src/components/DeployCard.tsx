// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, Divider, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import DEPLOY_CARD_HEIGHT from '../constants/layout';

export interface DeployCardProps {
  title?: string;
  description?: string;
  /** Replaces the plain title and description with a header of the card's own. */
  header?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  testId?: string;
}

/**
 * The outlined card the deploy path's columns share, with a header, body and footer. Every card has the
 * same height, so the path keeps its shape whatever a card shows; what does not fit scrolls within it.
 */
export default function DeployCard({
  title = undefined,
  description = undefined,
  header = undefined,
  children = undefined,
  footer = undefined,
  testId = undefined,
}: DeployCardProps): JSX.Element {
  return (
    <Card
      variant="outlined"
      data-testid={testId}
      sx={{display: 'flex', flexDirection: 'column', minWidth: 0, height: DEPLOY_CARD_HEIGHT}}
    >
      {header ?? (
        <Box sx={{p: 2.5}}>
          <Typography variant="h6">{title}</Typography>
          {description && (
            <Typography variant="body2" color="text.secondary" sx={{mt: 0.5}}>
              {description}
            </Typography>
          )}
        </Box>
      )}
      <Box sx={{flex: 1, minHeight: 0, overflowY: 'auto', display: 'flex', flexDirection: 'column'}}>{children}</Box>
      {footer && (
        <>
          <Divider />
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1, px: 2, py: 1.5}}>{footer}</Box>
        </>
      )}
    </Card>
  );
}
