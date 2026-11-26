// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Skeleton, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import CspScopeCard from './CspScopeCard';
import useGetCspConfig from '../../api/useGetCspConfig';
import useUpdateCspConfig from '../../api/useUpdateCspConfig';
import useCspDraft from '../../hooks/useCspDraft';

export default function CspSection(): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetCspConfig();
  const updateCsp = useUpdateCspConfig();
  const csp = useCspDraft(data);
  const {can} = useCapabilitiesContext();

  // Server-config error codes are shared across sections, so they resolve under `csp.errors`.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => {
      if (key.includes(':')) {
        return t(key, options);
      }
      return t(key.startsWith('errors.') ? `settings:csp.${key}` : `settings:${key}`, options);
    },
    [t],
  );

  // A previous save error is stale once the draft changes again.
  const clearSaveError = (): void => {
    if (updateCsp.isError) {
      updateCsp.reset();
    }
  };

  const handleSave = (): void => {
    if (csp.hasErrors) {
      return;
    }
    updateCsp.mutate({data: csp.buildPayload()}, {onSuccess: () => csp.reset()});
  };

  let directives: JSX.Element;
  if (isLoading || error) {
    directives = (
      <SettingsCard
        title={t('settings:csp.card.title', 'Directives')}
        description={t(
          'settings:csp.card.description',
          'The directives sent to the Console and Gate. Changing a directive overrides its default value.',
        )}
      >
        {isLoading ? (
          <Stack spacing={1}>
            <Skeleton variant="rounded" height={40} />
            <Skeleton variant="rounded" height={40} />
          </Stack>
        ) : (
          error && (
            <QueryErrorNotice
              error={error}
              t={tForErrors}
              variant="inline"
              fallbackKey="settings:csp.load.error"
              fallbackDefaultValue="Failed to load the content security policy."
              onRetry={() => void refetch()}
            />
          )
        )}
      </SettingsCard>
    );
  } else {
    directives = (
      <>
        {csp.scopes.map((scope) => (
          <CspScopeCard
            key={scope}
            scope={scope}
            rows={csp.rowsFor(scope)}
            availableDirectives={csp.availableDirectives(scope)}
            errors={csp.errors}
            reportOnly={csp.reportOnly}
            reportUri={data?.merged.reportUri}
            onSourcesChange={(directive, sources) => {
              clearSaveError();
              csp.setSources(scope, directive, sources);
            }}
            onDirectiveChange={(draftId, directive) => {
              clearSaveError();
              csp.changeDirective(scope, draftId, directive);
            }}
            onRemove={(draftId) => {
              clearSaveError();
              csp.removeRow(scope, draftId);
            }}
            onAdd={() => {
              clearSaveError();
              csp.addRow(scope);
            }}
          />
        ))}
        <Typography variant="caption" color="text.secondary">
          {t(
            'settings:csp.header.nonceHint',
            "Each response also adds a one-time nonce to script-src, and to style-src-elem when it is set, unless the directive allows 'unsafe-inline'.",
          )}
        </Typography>
      </>
    );
  }

  return (
    <>
      <Stack spacing={3}>
        {data && (
          <SettingsCard
            title={t('settings:csp.reportOnly.title', 'Report only')}
            description={t(
              'settings:csp.reportOnly.description',
              'When on, browsers report content the policy would block instead of blocking it. Use it to test changes before enforcing them.',
            )}
            enabled={csp.reportOnly}
            onToggle={(enabled) => {
              clearSaveError();
              csp.setReportOnly(enabled);
            }}
          >
            {null}
          </SettingsCard>
        )}
        {directives}
      </Stack>
      {csp.dirty && (
        <UnsavedChangesBar
          message={t('settings:csp.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('settings:csp.reset', 'Reset')}
          saveLabel={t('settings:csp.save', 'Save changes')}
          savingLabel={t('settings:csp.saving', 'Saving...')}
          isSaving={updateCsp.isPending}
          saveDisabled={csp.hasErrors || !can('serverconfig', 'update')}
          error={
            updateCsp.error
              ? getErrorMessage(
                  updateCsp.error,
                  tForErrors,
                  'csp.save.error',
                  'Failed to update the content security policy.',
                )
              : undefined
          }
          onReset={() => {
            clearSaveError();
            csp.reset();
          }}
          onSave={handleSave}
        />
      )}
    </>
  );
}
