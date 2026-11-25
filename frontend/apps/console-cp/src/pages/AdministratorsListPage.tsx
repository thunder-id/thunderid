// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UsersListPage} from '@thunderid/configure-users';
import {useProjectScope} from '@thunderid/contexts';
import {Alert} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * The Control Plane's users: the administrators who sign in to this console. The users of the
 * gateways it deploys to are managed in each gateway. Inside a project, the page lists the project's
 * users instead.
 */
export default function AdministratorsListPage(): JSX.Element {
  const {t} = useTranslation('users');
  const {projectId} = useProjectScope();
  if (projectId) {
    return <UsersListPage />;
  }
  return (
    <UsersListPage
      title={t('administratorsTitle', 'Administrators')}
      notice={
        <Alert severity="info" sx={{mb: 3}} data-testid="administrators-notice">
          {t(
            'administratorsNotice',
            "Administrators sign in to this console to manage the configuration and deploy it. This list also shows the users of each project, which a deploy of the project carries to its gateways. Users added in a gateway's admin portal are managed there.",
          )}
        </Alert>
      }
    />
  );
}
