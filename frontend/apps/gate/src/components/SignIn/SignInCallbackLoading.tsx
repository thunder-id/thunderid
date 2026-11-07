// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Loading state shown while sign-in completes after returning from a social identity provider, in
 * place of the sign-in card, which has nothing to show until the flow responds. It is the same
 * spinner, in the same place, as the page and theme loaders before it, so they read as one.
 */
export default function SignInCallbackLoading(): JSX.Element {
  const {t} = useTranslation();

  return (
    <Box
      role="status"
      className={cn('SignInCallbackLoading--root')}
      sx={{position: 'fixed', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center'}}
    >
      <CircularProgress aria-label={t('signin:callback.loading', 'Signing you in')} />
    </Box>
  );
}
