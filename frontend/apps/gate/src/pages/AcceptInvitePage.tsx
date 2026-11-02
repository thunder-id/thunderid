// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {AuthPageLayout} from '@thunderid/design';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AcceptInviteBox from '../components/AcceptInvite/AcceptInviteBox';

/**
 * AcceptInvitePage - Page for end users to accept an invite and set their password.
 *
 * This page is accessed via an invite link containing executionId and inviteToken query parameters.
 * Example: /invite?executionId=xxx&inviteToken=yyy
 */
export default function AcceptInvitePage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <AuthPageLayout isLoading={false}>
      <DocumentTitle title={t('auth:acceptInvite', 'Accept invite')} />
      <AcceptInviteBox />
    </AuthPageLayout>
  );
}
