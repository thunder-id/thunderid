// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {RoleAssignments, RolePermissions, useGetRole} from '@thunderid/react';
import {Button, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useParams} from 'react-router';
import useManageRoleRoutes from '../hooks/useManageRoleRoutes';

export interface RoleDetailsPageProps {
  /**
   * Whether admins may assign the role and take it away.
   */
  editable?: boolean;
}

/**
 * Shows the role named by the `id` route parameter, with its permissions and who holds it.
 */
export default function RoleDetailsPage({editable = false}: RoleDetailsPageProps): JSX.Element {
  const {t} = useTranslation();
  const routes = useManageRoleRoutes();
  const {id = ''} = useParams();
  const {data: role} = useGetRole(id);

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
      <Stack spacing={1}>
        <Typography variant="h4" component="h1">
          {role?.name ?? ''}
        </Typography>
        {role?.description && <Typography color="text.secondary">{role.description}</Typography>}
        <RolePermissions roleId={id} />
        <RoleAssignments roleId={id} editable={editable} />
      </Stack>
    </Stack>
  );
}
