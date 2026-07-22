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

import {SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, Divider, Skeleton, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {InfoIcon, Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import OriginRow from './OriginRow';
import useGetCorsConfig from '../../api/useGetCorsConfig';
import useUpdateCorsConfig from '../../api/useUpdateCorsConfig';
import useAllowedOriginsDraft from '../../hooks/useAllowedOriginsDraft';
import type {AllowedOrigin} from '../../models/responses';

const ROW_ACTION_WIDTH = 40;

/** Renders an allowed origin for display: a literal string as-is, a regex entry as its pattern. */
function originText(entry: AllowedOrigin): string {
  return typeof entry === 'string' ? entry : entry.regex;
}

/** A single non-editable origin row: a muted read-only field plus a spacer that aligns with editable rows. */
function OriginDisplayRow({value}: {value: string}): JSX.Element {
  return (
    <Stack direction="row" spacing={1} alignItems="center">
      <TextField
        fullWidth
        size="small"
        value={value}
        slotProps={{input: {readOnly: true}}}
        sx={{flex: 1, opacity: 0.65}}
      />
      <Box aria-hidden sx={{width: ROW_ACTION_WIDTH, flex: 'none'}} />
    </Stack>
  );
}

export default function CorsSection(): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, error} = useGetCorsConfig();
  const updateCors = useUpdateCorsConfig();
  const origins = useAllowedOriginsDraft(data);

  const readOnlyOrigins: AllowedOrigin[] = data?.readOnly.allowedOrigins ?? [];
  const hasReadOnlyOrigins: boolean = readOnlyOrigins.length > 0;

  const handleSave = (): void => {
    if (!origins.validateAll()) {
      return;
    }
    updateCors.mutate(
      {data: origins.buildPayload()},
      {
        onSuccess: () => {
          origins.reset();
        },
      },
    );
  };

  let body: JSX.Element;
  if (isLoading) {
    body = (
      <Stack spacing={1}>
        <Skeleton variant="rounded" height={40} />
        <Skeleton variant="rounded" height={40} />
      </Stack>
    );
  } else if (error) {
    body = <Alert severity="error">{getErrorMessage(error, t, 'settings:cors.load.error')}</Alert>;
  } else {
    body = (
      <>
        <Stack spacing={1}>
          {readOnlyOrigins.map((entry, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <OriginDisplayRow key={`readonly-${index}`} value={originText(entry)} />
          ))}
          {origins.draft.map((value, index) => (
            <OriginRow
              // eslint-disable-next-line react/no-array-index-key
              key={`origin-${index}`}
              value={value}
              error={origins.errors[index]}
              placeholder={t('settings:cors.originPlaceholder')}
              removeLabel={t('settings:cors.removeOrigin')}
              onChange={(next) => origins.changeRow(index, next)}
              onBlur={() => origins.blurRow(index)}
              onRemove={() => origins.removeRow(index)}
            />
          ))}
        </Stack>

        <Button variant="outlined" startIcon={<Plus size={18} />} onClick={origins.addRow} sx={{mt: 2}}>
          {t('settings:cors.addOrigin')}
        </Button>

        {hasReadOnlyOrigins && (
          <>
            <Divider sx={{mt: 2, mb: 1.5}} />
            <Stack direction="row" spacing={1} alignItems="flex-start">
              <Box aria-hidden sx={{flex: 'none', display: 'inline-flex', mt: '2px', color: 'text.secondary'}}>
                <InfoIcon size={16} />
              </Box>
              <Typography variant="body2" color="text.secondary">
                {t('settings:cors.readOnlyHint')}
              </Typography>
            </Stack>
          </>
        )}
      </>
    );
  }

  return (
    <>
      <SettingsCard title={t('settings:cors.card.title')} description={t('settings:cors.card.description')}>
        {body}
      </SettingsCard>
      {origins.dirty && (
        <UnsavedChangesBar
          message={t('settings:cors.unsavedChanges')}
          resetLabel={t('settings:cors.reset', 'Reset')}
          saveLabel={t('settings:cors.save', 'Save changes')}
          savingLabel={t('settings:cors.saving', 'Saving changes...')}
          isSaving={updateCors.isPending}
          saveDisabled={origins.hasErrors}
          onReset={origins.reset}
          onSave={handleSave}
        />
      )}
    </>
  );
}
