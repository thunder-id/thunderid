// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SignIn from '../components/SignIn/SignIn';

export default function SignInPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <DocumentTitle title={t('auth:signIn', 'Sign in')} />
      <SignIn />
    </>
  );
}
