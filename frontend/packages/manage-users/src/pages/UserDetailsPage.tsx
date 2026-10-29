// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UserDetails, type AdministrationMode} from '@thunderid/react';
import {Button, Stack} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useManageUserRoutes from '../hooks/useManageUserRoutes';

export interface UserDetailsPageProps {
  /**
   * How a delete runs. `flow`, the default, runs the administration flow the server configures for it, so
   * grants are revoked and sessions end first; with none configured, the API is called directly.
   */
  administrationMode?: AdministrationMode;
  deletable?: boolean;
  editable?: boolean;
  /**
   * Whether admins may suspend, unsuspend and unlock the user. Each action is offered only when the
   * server configures its administration flow, and reading that needs permission to read the server
   * configuration.
   */
  lifecycleActions?: boolean;
}

/**
 * Shows the user named by the `id` route parameter. Deleting the user returns to the users list.
 */
export default function UserDetailsPage({
  administrationMode = 'flow',
  deletable = false,
  editable = false,
  lifecycleActions = false,
}: UserDetailsPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageUserRoutes();
  const {id = ''} = useParams();

  return (
    <Stack spacing={2}>
      <Button
        component={Link}
        to={routes.list()}
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        sx={{alignSelf: 'flex-start'}}
      >
        {t('manage:actions.back', 'Back')}
      </Button>
      <UserDetails
        userId={id}
        administrationMode={administrationMode}
        deletable={deletable}
        editable={editable}
        lifecycleActions={lifecycleActions}
        onDeleted={() => {
          void navigate(routes.list(), {replace: true});
        }}
      />
    </Stack>
  );
}
