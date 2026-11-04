// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GroupDetails, type AdministrationMode} from '@thunderid/react';
import {Button, Stack} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useManageGroupRoutes from '../hooks/useManageGroupRoutes';

export interface GroupDetailsPageProps {
  /**
   * How the group is deleted and a member removed. `flow`, the default, runs the administration flow the server configures for it, so
   * grants are revoked and sessions end first; with none configured, the API is called directly.
   */
  administrationMode?: AdministrationMode;
  /**
   * Whether admins may delete the group.
   */
  deletable?: boolean;
  /**
   * Whether admins may add and remove members.
   */
  editable?: boolean;
}

export default function GroupDetailsPage({
  administrationMode = 'flow',
  deletable = false,
  editable = false,
}: GroupDetailsPageProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useManageGroupRoutes();
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
      <GroupDetails
        groupId={id}
        administrationMode={administrationMode}
        deletable={deletable}
        editable={editable}
        onDeleted={() => {
          void navigate(routes.list(), {replace: true});
        }}
      />
    </Stack>
  );
}
