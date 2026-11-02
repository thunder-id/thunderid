// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {RoleList} from '@thunderid/react';
import {Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import useManageRoleRoutes from '../hooks/useManageRoleRoutes';

export interface RolesListPageProps {
  /**
   * Whether admins may add roles from this page.
   */
  canAdd?: boolean;
  description?: string;
  title?: string;
}

/**
 * Lists the roles the signed-in admin can manage. Choosing one opens its details.
 */
export default function RolesListPage({
  canAdd = false,
  description = undefined,
  title = undefined,
}: RolesListPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageRoleRoutes();

  return (
    <RoleList
      title={title ?? t('manage:nav.roles', 'Roles')}
      description={description}
      actions={
        canAdd ? (
          <Button component={Link} to={routes.create()} variant="contained" startIcon={<Plus size={16} />}>
            {t('manage:actions.add.roles', t('manage:actions.add', 'Add'))}
          </Button>
        ) : undefined
      }
      onSelect={({id}: {id: string}) => {
        void navigate(routes.details(id));
      }}
    />
  );
}
