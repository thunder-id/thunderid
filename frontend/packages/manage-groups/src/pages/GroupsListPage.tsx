// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GroupList} from '@thunderid/react';
import {Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import useManageGroupRoutes from '../hooks/useManageGroupRoutes';

export interface GroupsListPageProps {
  /**
   * Whether admins may add groups from this page.
   */
  canAdd?: boolean;
  description?: string;
  title?: string;
}

/**
 * Lists the groups the signed-in admin can manage. Choosing one opens its details.
 */
export default function GroupsListPage({
  canAdd = false,
  description = undefined,
  title = undefined,
}: GroupsListPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageGroupRoutes();

  return (
    <GroupList
      title={title ?? t('manage:nav.groups', 'Groups')}
      description={description}
      actions={
        canAdd ? (
          <Button component={Link} to={routes.create()} variant="contained" startIcon={<Plus size={16} />}>
            {t('manage:actions.add.groups', t('manage:actions.add', 'Add'))}
          </Button>
        ) : undefined
      }
      onSelect={({id}: {id: string}) => {
        void navigate(routes.details(id));
      }}
    />
  );
}
