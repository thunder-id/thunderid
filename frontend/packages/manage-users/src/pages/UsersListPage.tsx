// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UserList} from '@thunderid/react';
import {Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import useManageUserRoutes from '../hooks/useManageUserRoutes';

export interface UsersListPageProps {
  /**
   * How admins add people from this page: `create` adds them directly, `invite` sends them an
   * invitation. Leave it out to hide the button.
   */
  addAction?: 'create' | 'invite';
  description?: string;
  title?: string;
}

/**
 * Lists the users the signed-in admin can manage. Choosing one opens its details.
 */
export default function UsersListPage({
  addAction = undefined,
  description = undefined,
  title = undefined,
}: UsersListPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageUserRoutes();

  return (
    <UserList
      title={title ?? t('manage:nav.users', 'Users')}
      description={description}
      actions={
        addAction ? (
          <Button component={Link} to={routes.create()} variant="contained" startIcon={<Plus size={16} />}>
            {t(
              addAction === 'invite' ? 'manage:actions.invite.users' : 'manage:actions.add.users',
              t('manage:actions.add', 'Add'),
            )}
          </Button>
        ) : undefined
      }
      onSelect={({id}: {id: string}) => {
        void navigate(routes.details(id));
      }}
    />
  );
}
