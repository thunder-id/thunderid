// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import Device from '../components/Device/Device';

export default function DevicePage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <DocumentTitle title={t('auth:signIn', 'Sign in')} />
      <Device />
    </>
  );
}
