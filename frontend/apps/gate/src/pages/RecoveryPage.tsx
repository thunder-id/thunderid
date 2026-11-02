// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import Recovery from '../components/Recovery/Recovery';

export default function RecoveryPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <DocumentTitle title={t('auth:resetPassword', 'Reset password')} />
      <Recovery />
    </>
  );
}
