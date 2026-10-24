// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SignOut from '../components/SignOut/SignOut';

export default function SignOutPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <DocumentTitle title={t('common:actions.signOut', 'Sign out')} />
      <SignOut />
    </>
  );
}
