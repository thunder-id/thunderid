// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UsersListPage} from '@thunderid/configure-users';
import {Alert} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * The Control Plane's users: the administrators who sign in to this console. The users of the
 * gateways it deploys to are managed in each gateway.
 */
export default function AdministratorsListPage(): JSX.Element {
  const {t} = useTranslation('users');
  return (
    <UsersListPage
      title={t('administratorsTitle', 'Administrators')}
      notice={
        <Alert severity="info" sx={{mb: 3}} data-testid="administrators-notice">
          {t(
            'administratorsNotice',
            'Administrators sign in to this console to manage the configuration and deploy it. They are not users of your gateways: those are managed in each gateway, from its admin portal.',
          )}
        </Alert>
      }
    />
  );
}
