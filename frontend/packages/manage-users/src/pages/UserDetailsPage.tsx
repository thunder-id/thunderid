// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UserDetails} from '@thunderid/react';
import {Button, Stack} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useManageUserRoutes from '../hooks/useManageUserRoutes';

export interface UserDetailsPageProps {
  deletable?: boolean;
  editable?: boolean;
}

/**
 * Shows the user named by the `id` route parameter. Deleting the user returns to the users list.
 */
export default function UserDetailsPage({deletable = false, editable = false}: UserDetailsPageProps): JSX.Element {
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
        deletable={deletable}
        editable={editable}
        onDeleted={() => {
          void navigate(routes.list(), {replace: true});
        }}
      />
    </Stack>
  );
}
