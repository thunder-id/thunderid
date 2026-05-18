/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {cn} from '@thunderid/utils';
import {ColorSchemeImage, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';

export default function Error(): JSX.Element {
  const [searchParams] = useSearchParams();
  const {t} = useTranslation();

  const errorCode = searchParams.get('errorCode') ?? '';
  const isInvalidRequest = errorCode === 'invalid_request';

  const errorTitle = isInvalidRequest ? t('errors:page.invalidRequest.title') : t('errors:page.defaultTitle');
  const errorDescription = isInvalidRequest
    ? t('errors:page.invalidRequest.description')
    : (searchParams.get('errorMessage') ?? t('errors:page.defaultDescription'));
  const errorImagePublicPath = '/assets/images/error-500.svg';
  const errorImageInvertedPublicPath = '/assets/images/error-500-inverted.svg';

  return (
    <Stack
      direction="column"
      component="main"
      className={cn('Error--root')}
      sx={[
        {
          justifyContent: 'center',
          height: 'calc((1 - var(--template-frame-height, 0)) * 100%)',
          minHeight: '100%',
        },
      ]}
    >
      <Stack gap={5}>
        <ColorSchemeImage
          src={{
            light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
            dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
          }}
          alt={{light: 'Logo (Light)', dark: 'Logo (Dark)'}}
          height={40}
          width="auto"
        />
        <ColorSchemeImage
          src={{
            light: `${import.meta.env.BASE_URL}${errorImagePublicPath}`,
            dark: `${import.meta.env.BASE_URL}${errorImageInvertedPublicPath}`,
          }}
          alt={{light: 'Error Image (Light)', dark: 'Error Image (Dark)'}}
          height={400}
          width="auto"
        />
        <Stack sx={{flexDirection: 'column', alignSelf: 'center'}}>
          <Typography component="h1" variant="h2" color="error" sx={{mb: 1}}>
            {errorTitle}
          </Typography>
          <Typography component="p" variant="body1" color="text.secondary" textAlign="center">
            {errorDescription}
          </Typography>
        </Stack>
      </Stack>
    </Stack>
  );
}
