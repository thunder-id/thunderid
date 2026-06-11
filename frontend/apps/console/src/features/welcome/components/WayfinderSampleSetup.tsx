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

import {useConfig} from '@thunderid/contexts';
import {Box, Divider, Stack, Typography} from '@wso2/oxygen-ui';
import {CheckCircle, ChevronRight, Database, Download, Play, Settings} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import TerminalBlock from './TerminalBlock';
import WayfinderConfigImport from './WayfinderConfigImport';
import WayfinderSampleDownload from './WayfinderSampleDownload';
import getWayfinderConfiguredStorageKey from '../utils/getWayfinderConfiguredStorageKey';
import getWayfinderSetupExpandedStorageKey from '../utils/getWayfinderSetupExpandedStorageKey';

export default function WayfinderSampleSetup(): JSX.Element {
  const {t} = useTranslation(['common']);
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const importedKey = getWayfinderConfiguredStorageKey(productName);
  const expandedKey = getWayfinderSetupExpandedStorageKey(productName);
  const releasesUrl = config.brand.documentation?.releasesUrl ?? '';

  const [isDone, setIsDone] = useState(() => !!sessionStorage.getItem(importedKey));
  const [expanded, setExpanded] = useState(() => {
    const saved = sessionStorage.getItem(expandedKey);
    if (saved !== null) return saved === 'true';
    return !sessionStorage.getItem(importedKey);
  });

  const handleImportSuccess = (): void => {
    setIsDone(true);
    sessionStorage.setItem(expandedKey, 'false');
  };

  const toggle = (): void =>
    setExpanded((v) => {
      sessionStorage.setItem(expandedKey, String(!v));
      return !v;
    });

  const subStepNumber = (n: number): JSX.Element => (
    <Box
      sx={{
        width: 24,
        height: 24,
        borderRadius: '50%',
        bgcolor: 'action.selected',
        color: 'text.primary',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: '0.75rem',
        fontWeight: 700,
        flexShrink: 0,
        mt: 0.25,
      }}
    >
      {n}
    </Box>
  );

  return (
    <Box
      sx={{border: '1px solid', borderColor: isDone ? 'success.light' : 'divider', borderRadius: 2, overflow: 'hidden'}}
    >
      {/* Header */}
      <Box
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={toggle}
        onKeyDown={(e: React.KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            toggle();
          }
        }}
        sx={{
          p: 2.5,
          display: 'flex',
          alignItems: 'center',
          gap: 2,
          bgcolor: 'action.selected',
          cursor: 'pointer',
          userSelect: 'none',
          '&:hover': {bgcolor: 'action.hover'},
          borderBottom: expanded ? '1px solid' : 'none',
          borderColor: 'divider',
        }}
      >
        <Box
          sx={{
            width: 40,
            height: 40,
            borderRadius: 2,
            bgcolor: isDone ? 'success.lighter' : 'background.paper',
            color: isDone ? 'success.main' : 'text.secondary',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
          }}
        >
          {isDone ? <CheckCircle size={22} /> : <Settings size={22} />}
        </Box>
        <Box sx={{flex: 1, minWidth: 0}}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Typography variant="subtitle1" fontWeight={600}>
              {t('common:welcome.wayfinderSampleSetup.title')}
            </Typography>
            <Typography
              variant="caption"
              sx={{px: 1, py: 0.25, borderRadius: 1, bgcolor: 'action.hover', color: 'text.secondary', flexShrink: 0}}
            >
              {t('common:welcome.wayfinderSampleSetup.oneTimeSetup')}
            </Typography>
          </Stack>
          {isDone && !expanded && (
            <Typography variant="caption" color="success.main">
              {t('common:welcome.wayfinderSampleSetup.setupComplete')}
            </Typography>
          )}
        </Box>
        <ChevronRight
          size={18}
          style={{
            transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
            transition: 'transform 0.2s',
            flexShrink: 0,
            color: 'inherit',
            opacity: 0.6,
          }}
        />
      </Box>

      {/* Sub-steps */}
      {expanded && (
        <Stack divider={<Divider />}>
          {/* 1 — Download */}
          <Box sx={{p: 2.5}}>
            <Stack direction="row" spacing={2} alignItems="flex-start">
              {subStepNumber(1)}
              <Box sx={{flex: 1}}>
                <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
                  <Box sx={{color: 'text.secondary', display: 'flex'}}>
                    <Download size={16} />
                  </Box>
                  <Typography variant="subtitle2" fontWeight={600}>
                    {t('common:welcome.wayfinderSampleSetup.steps.getSample.title')}
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                  <Trans
                    i18nKey="common:welcome.wayfinderSampleSetup.steps.getSample.description"
                    components={{strong: <strong />}}
                  />
                </Typography>
                {releasesUrl ? <WayfinderSampleDownload releasesUrl={releasesUrl} /> : null}
              </Box>
            </Stack>
          </Box>

          {/* 2 — Configure ThunderID */}
          <Box sx={{p: 2.5}}>
            <Stack direction="row" spacing={2} alignItems="flex-start">
              {subStepNumber(2)}
              <Box sx={{flex: 1}}>
                <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
                  <Box sx={{color: 'text.secondary', display: 'flex'}}>
                    <Database size={16} />
                  </Box>
                  <Typography variant="subtitle2" fontWeight={600}>
                    {t('common:welcome.wayfinderSampleSetup.steps.configure.title', {productName})}
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                  {t('common:welcome.wayfinderSampleSetup.steps.configure.description', {productName})}
                </Typography>
                <WayfinderConfigImport onSuccess={handleImportSuccess} />
              </Box>
            </Stack>
          </Box>

          {/* 3 — Run */}
          <Box sx={{p: 2.5}}>
            <Stack direction="row" spacing={2} alignItems="flex-start">
              {subStepNumber(3)}
              <Box sx={{flex: 1}}>
                <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
                  <Box sx={{color: 'text.secondary', display: 'flex'}}>
                    <Play size={16} />
                  </Box>
                  <Typography variant="subtitle2" fontWeight={600}>
                    {t('common:welcome.wayfinderSampleSetup.steps.run.title')}
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                  {t('common:welcome.wayfinderSampleSetup.steps.run.description')}
                </Typography>
                <TerminalBlock command={'npm i && npm run dev'} />
              </Box>
            </Stack>
          </Box>
        </Stack>
      )}
    </Box>
  );
}
