// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GroupMembers, useGetGroup} from '@thunderid/react';
import {Button, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useParams} from 'react-router';
import useManageGroupRoutes from '../hooks/useManageGroupRoutes';

export interface GroupDetailsPageProps {
  /**
   * Whether admins may add and remove members.
   */
  editable?: boolean;
}

/**
 * Shows the group named by the `id` route parameter, with its members.
 */
export default function GroupDetailsPage({editable = false}: GroupDetailsPageProps): JSX.Element {
  const {t} = useTranslation();
  const routes = useManageGroupRoutes();
  const {id = ''} = useParams();
  const {data: group} = useGetGroup(id);

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
          {group?.name ?? ''}
        </Typography>
        {group?.description && <Typography color="text.secondary">{group.description}</Typography>}
        <GroupMembers groupId={id} editable={editable} />
      </Stack>
    </Stack>
  );
}
