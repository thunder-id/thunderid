// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OrganizationAccessSettings, type OrganizationAccessState} from '@thunderid/configure-resource-sharing';
import type {User} from '@thunderid/types';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ResharesList from './ResharesList';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface UserOrganizationAccessProps {
  user: User;
  /** The user's Organization Access state, held by the page so the draft survives tab switches. */
  access: OrganizationAccessState;
}

/**
 * The Organization Access tab of a user: the shared Organization Access screen for the share the user's own unit
 * makes, and the shares other units made of the user beneath it.
 */
export default function UserOrganizationAccess({user, access}: UserOrganizationAccessProps): JSX.Element {
  const {t} = useTranslation('users');
  const userName = user.display ?? user.id;

  return (
    <Stack spacing={3}>
      <OrganizationAccessSettings
        access={access}
        ownerOuId={user.ouId}
        resourceName={userName}
        // A declared user's own sharing is set in its file; the server refuses it here.
        readOnly={user.isReadOnly === true}
        errorNamespace="users"
        resolveErrorMessage={getUserErrorMessage}
        description={t(
          'sharing.description',
          '{{name}} can sign in to the applications of a granted organization unit.',
          {name: userName},
        )}
      />
      <ResharesList userName={userName} resourcePath={`users/${user.id}`} />
    </Stack>
  );
}
