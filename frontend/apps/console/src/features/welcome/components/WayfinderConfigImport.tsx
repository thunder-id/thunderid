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
import {Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {CheckCircle, Database, RefreshCw, XCircle} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import useImportConfiguration from '../../import-export/api/useImportConfiguration';
import type {ImportResponse} from '../../import-export/models/import-configuration';
import {useGetSampleBundle} from '../api/useGetSampleBundles';
import getWayfinderConfiguredStorageKey from '../utils/getWayfinderConfiguredStorageKey';

const WAYFINDER_BUNDLE_KEY = 'wayfinder';

type Status = 'idle' | 'importing' | 'success' | 'alreadyDone' | 'error';

/**
 * Parses the content of an .env file and returns an object mapping variable names to their values.
 *
 * @param content - The string content of the .env file to parse.
 * @returns An object where keys are environment variable names and values are their corresponding values.
 */
function parseEnvFile(content: string): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const line of content.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const eq = trimmed.indexOf('=');
    if (eq === -1) continue;
    vars[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1).trim();
  }
  return vars;
}

/**
 * Formats a timestamp string into a human-readable date format.
 *
 * @param ts - The timestamp string to format.
 * @returns A formatted date string or an empty string if the timestamp is invalid.
 */
function formatImportedDate(ts: string): string {
  const date = new Date(Number(ts));
  if (isNaN(date.getTime())) return '';
  return date.toLocaleDateString(undefined, {day: 'numeric', month: 'short', year: 'numeric'});
}

interface WayfinderConfigImportProps {
  onSuccess?: () => void;
}

/**
 * Component that provides functionality to import a predefined set of configurations (Wayfinder bundle)
 * into the application.
 *
 * @param onSuccess - Optional callback function to be called after a successful import.
 * @returns A JSX element that renders the import UI and handles the import process.
 */
export default function WayfinderConfigImport({onSuccess = undefined}: WayfinderConfigImportProps): JSX.Element {
  const {t} = useTranslation(['common']);
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const importedKey = getWayfinderConfiguredStorageKey(productName);
  const {mutateAsync: importConfig} = useImportConfiguration();
  const bundle = useGetSampleBundle(WAYFINDER_BUNDLE_KEY);

  const [status, setStatus] = useState<Status>(() => {
    const ts = sessionStorage.getItem(importedKey);
    return ts ? 'alreadyDone' : 'idle';
  });
  const [result, setResult] = useState<ImportResponse | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const previousTs = sessionStorage.getItem(importedKey);

  const handleImport = async (): Promise<void> => {
    if (!bundle?.configs.declarative) {
      setErrorMsg(t('common:welcome.wayfinderFolderImport.errors.importFailed'));
      setStatus('error');
      return;
    }
    setStatus('importing');
    setErrorMsg(null);

    try {
      const variables = bundle.configs.env ? parseEnvFile(bundle.configs.env) : undefined;
      const response = await importConfig({
        content: bundle.configs.declarative,
        variables,
        options: {upsert: true},
      });
      setResult(response);
      if (response.summary.failed > 0) {
        setErrorMsg(t('common:welcome.wayfinderFolderImport.errors.partialFailure', {count: response.summary.failed}));
        setStatus('error');
      } else {
        sessionStorage.setItem(importedKey, Date.now().toString());
        setStatus('success');
        onSuccess?.();
      }
    } catch {
      setErrorMsg(t('common:welcome.wayfinderFolderImport.errors.importFailed'));
      setStatus('error');
    }
  };

  const handleReset = (): void => {
    setStatus('idle');
    setResult(null);
    setErrorMsg(null);
  };

  if (status === 'importing') {
    return (
      <Stack direction="row" spacing={1.5} alignItems="center">
        <CircularProgress size={18} />
        <Typography variant="body2" color="text.secondary">
          {t('common:welcome.wayfinderFolderImport.status.importing')}
        </Typography>
      </Stack>
    );
  }

  if (status === 'alreadyDone') {
    return (
      <Stack spacing={1}>
        <Stack direction="row" spacing={1} alignItems="center">
          <CheckCircle size={20} style={{color: 'var(--oxygen-palette-success-main)'}} />
          <Typography variant="body2" fontWeight={600} color="success.main">
            {t('common:welcome.wayfinderFolderImport.status.alreadyDone', {productName})}
          </Typography>
        </Stack>
        {previousTs && (
          <Typography variant="caption" color="text.secondary">
            {t('common:welcome.wayfinderFolderImport.status.lastImported', {date: formatImportedDate(previousTs)})}
          </Typography>
        )}
        <Box>
          <Button
            variant="outlined"
            size="small"
            startIcon={<RefreshCw size={13} />}
            sx={{px: 2, fontSize: '0.75rem'}}
            onClick={handleReset}
          >
            {t('common:welcome.wayfinderFolderImport.actions.reconfigure')}
          </Button>
        </Box>
      </Stack>
    );
  }

  if (status === 'success' && result) {
    return (
      <Stack spacing={1}>
        <Stack direction="row" spacing={1} alignItems="center">
          <CheckCircle size={20} style={{color: 'var(--oxygen-palette-success-main)'}} />
          <Typography variant="body2" fontWeight={600} color="success.main">
            {t('common:welcome.wayfinderFolderImport.status.success', {productName})}
          </Typography>
        </Stack>
        <Typography variant="caption" color="text.secondary">
          {t('common:welcome.wayfinderFolderImport.status.resourcesImported', {count: result.summary.imported})}
        </Typography>
      </Stack>
    );
  }

  return (
    <Stack spacing={1.5}>
      <Box>
        <Button
          variant="contained"
          size="small"
          startIcon={<Database size={16} />}
          onClick={() => void handleImport()}
          disabled={!bundle?.configs.declarative}
        >
          {t('common:welcome.wayfinderFolderImport.actions.importConfig', {productName})}
        </Button>
      </Box>

      {status === 'error' && (
        <Stack spacing={1}>
          <Stack direction="row" spacing={1} alignItems="center">
            <XCircle size={16} style={{color: 'var(--oxygen-palette-error-main)', flexShrink: 0}} />
            <Typography variant="caption" color="error.main">
              {errorMsg}
            </Typography>
          </Stack>
          {result?.results
            .filter((r) => r.status === 'failed')
            .map((r) => (
              <Typography
                key={`${r.resourceType}-${r.resourceId ?? r.resourceName ?? r.message}`}
                variant="caption"
                color="error.main"
                sx={{pl: 3, display: 'block'}}
              >
                {r.resourceType}
                {r.resourceName ? ` · ${r.resourceName}` : ''}: {r.message}
              </Typography>
            ))}
        </Stack>
      )}
    </Stack>
  );
}
