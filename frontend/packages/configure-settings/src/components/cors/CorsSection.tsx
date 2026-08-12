// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Button, Divider, Skeleton, Stack, Typography} from '@wso2/oxygen-ui';
import {InfoIcon, Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import AllowedOriginRow from './AllowedOriginRow';
import useGetCorsConfig from '../../api/useGetCorsConfig';
import useUpdateCorsConfig from '../../api/useUpdateCorsConfig';
import useAllowedOriginsDraft from '../../hooks/useAllowedOriginsDraft';
import {toRows} from '../../utils/allowedOriginRows';

export default function CorsSection(): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetCorsConfig();
  const updateCors = useUpdateCorsConfig();
  const origins = useAllowedOriginsDraft(data);

  // Resolves an error through the `settings` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `settings:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `settings:${key}`, options),
    [t],
  );

  const readOnlyEntriesKey = JSON.stringify(data?.readOnly.allowedOrigins ?? []);
  const readOnlyRows = useMemo(
    () => toRows(data?.readOnly.allowedOrigins ?? []),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [readOnlyEntriesKey],
  );
  const hasReadOnlyOrigins: boolean = readOnlyRows.length > 0;

  const rowLabels = {
    originPlaceholder: t('settings:cors.originPlaceholder', 'https://app.example.com'),
    regexPlaceholder: t('settings:cors.regexPlaceholder', '^https://[a-z0-9-]+\\.example\\.com$'),
    typeLabel: t('settings:cors.type.label', 'Entry type'),
    originOptionLabel: t('settings:cors.type.origin', 'Origin'),
    regexOptionLabel: t('settings:cors.type.regex', 'Regex'),
    removeLabel: t('settings:cors.removeOrigin', 'Remove origin'),
  };

  // A previous save error is stale once the draft changes again.
  const clearSaveError = (): void => {
    if (updateCors.isError) {
      updateCors.reset();
    }
  };

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
    body = (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="inline"
        fallbackKey="settings:cors.load.error"
        fallbackDefaultValue="Failed to load allowed origins."
        onRetry={() => void refetch()}
      />
    );
  } else {
    body = (
      <>
        <Stack spacing={1}>
          {readOnlyRows.map((row) => (
            <AllowedOriginRow
              key={row.id}
              locked
              type={row.type}
              value={row.value}
              lockedLabel={t('settings:cors.lockedOrigin', "Managed declaratively and can't be edited here.")}
              {...rowLabels}
            />
          ))}
          {origins.draft.map((row) => (
            <AllowedOriginRow
              key={row.id}
              testId="cors-origin-row"
              type={row.type}
              value={row.value}
              error={origins.errors[row.id]}
              warning={origins.warnings[row.id]}
              {...rowLabels}
              onTypeChange={(type) => {
                clearSaveError();
                origins.changeRowType(row.id, type);
              }}
              onChange={(next) => {
                clearSaveError();
                origins.changeRow(row.id, next);
              }}
              onBlur={() => origins.blurRow(row.id)}
              onRemove={() => {
                clearSaveError();
                origins.removeRow(row.id);
              }}
            />
          ))}
        </Stack>

        <Button
          variant="text"
          color="primary"
          startIcon={<Plus size={18} />}
          onClick={() => {
            clearSaveError();
            origins.addRow();
          }}
          sx={{mt: 2}}
        >
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
          message={t('settings:cors.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('settings:cors.reset', 'Reset')}
          saveLabel={t('settings:cors.save', 'Save changes')}
          savingLabel={t('settings:cors.saving', 'Saving...')}
          isSaving={updateCors.isPending}
          saveDisabled={origins.hasErrors}
          error={
            updateCors.error
              ? getErrorMessage(updateCors.error, tForErrors, 'cors.save.error', 'Failed to update allowed origins.')
              : undefined
          }
          onReset={() => {
            clearSaveError();
            origins.reset();
          }}
          onSave={handleSave}
        />
      )}
    </>
  );
}
