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

import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {Download} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import useWayfinderReleases from '../api/useWayfinderReleases';

const PATTERN = /^sample-app-wayfinder-[0-9A-Za-z.+-]+\.zip$/i;

export default function WayfinderSampleDownload({releasesUrl}: {releasesUrl: string}): JSX.Element | null {
  const {t} = useTranslation(['common']);
  const {data, isError: errored} = useWayfinderReleases(releasesUrl);

  const release = data ? (data.latestRelease ?? data.releases?.[0] ?? null) : null;
  const asset = useMemo(() => release?.assets.find((a) => PATTERN.test(a.name)) ?? null, [release]);

  if (errored || !asset) return null;

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 2,
        p: 2,
        flexWrap: 'wrap',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
      }}
    >
      <Stack direction="column" spacing={0.25}>
        <Typography variant="body2" fontWeight={500}>
          {asset.name}
        </Typography>
        {asset.sizeLabel && (
          <Typography variant="caption" color="text.secondary">
            {asset.sizeLabel}
          </Typography>
        )}
      </Stack>
      <Button
        variant="contained"
        size="small"
        startIcon={<Download size={16} />}
        href={asset.downloadUrl}
        target="_blank"
        rel="noreferrer"
        component="a"
      >
        {t('common:welcome.wayfinderSampleDownload.downloadButton')}
      </Button>
    </Box>
  );
}
