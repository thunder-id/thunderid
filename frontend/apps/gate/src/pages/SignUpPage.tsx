// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SignUp from '../components/SignUp/SignUp';

export default function SignUpPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <DocumentTitle title={t('auth:signUp', 'Sign up')} />
      <SignUp />
    </>
  );
}
