// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AppBreadcrumbs, Box, IconButton} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {ComponentProps, JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface FullScreenHeaderProps {
  /** Where the page sits, as the console's full-screen pages show it. */
  breadcrumbs: ComponentProps<typeof AppBreadcrumbs>['items'];
  /** Leaves the full-screen view for the console. */
  onExit: () => void;
}

/**
 * The top of a full-screen deploy view, as the console's other full-screen pages have it: the way
 * out, then where the page sits.
 */
export default function FullScreenHeader({breadcrumbs, onExit}: FullScreenHeaderProps): JSX.Element {
  const {t} = useTranslation('deploy');
  return (
    <Box component="header" sx={{p: 4, display: 'flex', alignItems: 'center', gap: 2, flexShrink: 0}}>
      <IconButton
        onClick={onExit}
        aria-label={t('header.exit', 'Exit')}
        sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
      >
        <X size={24} />
      </IconButton>
      <AppBreadcrumbs items={breadcrumbs} />
    </Box>
  );
}
