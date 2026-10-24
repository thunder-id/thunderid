// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';
import Error from '../components/Error/Error';

export default function ErrorPage(): JSX.Element {
  const {t} = useTranslation();
  const [searchParams] = useSearchParams();
  const isInvalidRequest = searchParams.get('errorCode') === 'invalid_request';

  return (
    <>
      <DocumentTitle
        title={isInvalidRequest ? t('errors:invalidRequest', 'Invalid request') : t('common:status.error', 'Error')}
      />
      <Error />
    </>
  );
}
