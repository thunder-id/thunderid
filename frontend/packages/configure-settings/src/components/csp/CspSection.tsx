// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField, QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Button, Divider, FormControl, FormLabel, MenuItem, Select, Skeleton, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import CspDirectiveRow, {type CspDirectiveRowAction} from './CspDirectiveRow';
import useGetCspConfig from '../../api/useGetCspConfig';
import useUpdateCspConfig from '../../api/useUpdateCspConfig';
import useCspDraft from '../../hooks/useCspDraft';
import {CSP_DEFAULT_SCOPE, type CspScopeRow} from '../../models/cspDirectiveRow';
import {rowSources} from '../../utils/cspDirectiveRows';
import buildCspHeader from '../../utils/cspHeader';

const SCOPE_LABEL_ID = 'csp-scope-label';

const SCOPE_SELECT_WIDTH = 260;

/**
 * Picks the trailing action for a row: a directive with no default value can be removed, and an
 * overridden one can be reset.
 */
function rowAction(row: CspScopeRow): CspDirectiveRowAction | undefined {
  if (row.draftId === undefined) {
    return undefined;
  }
  return row.base.length === 0 ? 'remove' : 'reset';
}

export default function CspSection(): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetCspConfig();
  const updateCsp = useUpdateCspConfig();
  const csp = useCspDraft(data);
  const {can} = useCapabilitiesContext();
  const [scope, setScope] = useState<string>(CSP_DEFAULT_SCOPE);

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

  const rows = csp.rowsFor(scope);

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

  const scopeLabel = (key: string): string =>
    key === CSP_DEFAULT_SCOPE ? t('settings:csp.scope.all', 'All pages') : key;

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
        fallbackKey="settings:csp.load.error"
        fallbackDefaultValue="Failed to load the content security policy."
        onRetry={() => void refetch()}
      />
    );
  } else {
    const availableDirectives = csp.availableDirectives(scope);
    body = (
      <>
        <FormControl size="small" sx={{mb: 3}}>
          <FormLabel id={SCOPE_LABEL_ID}>{t('settings:csp.scope.label', 'Applies to')}</FormLabel>
          <Select
            labelId={SCOPE_LABEL_ID}
            value={scope}
            onChange={(event) => setScope(event.target.value)}
            sx={{width: SCOPE_SELECT_WIDTH}}
          >
            {csp.scopes.map((key) => (
              <MenuItem key={key} value={key}>
                {scopeLabel(key)}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <Stack spacing={1}>
          {rows.map((row) => (
            <CspDirectiveRow
              // A default row keeps its key when it gains an override, so the field keeps focus.
              key={`${scope} ${row.base.length > 0 ? row.directive : row.draftId}`}
              testId="csp-directive-row"
              directive={row.directive}
              sources={rowSources(row)}
              directiveOptions={row.base.length === 0 ? availableDirectives : undefined}
              error={row.draftId ? csp.errors[row.draftId] : undefined}
              action={rowAction(row)}
              onDirectiveChange={(directive) => {
                clearSaveError();
                if (row.draftId) {
                  csp.changeDirective(scope, row.draftId, directive);
                }
              }}
              onSourcesChange={(sources) => {
                clearSaveError();
                csp.setSources(scope, row.directive, sources);
              }}
              onAction={() => {
                clearSaveError();
                if (row.draftId) {
                  csp.removeRow(scope, row.draftId);
                }
              }}
            />
          ))}
        </Stack>

        {availableDirectives.length > 0 && (
          <Button
            variant="text"
            color="primary"
            startIcon={<Plus size={18} />}
            onClick={() => {
              clearSaveError();
              csp.addRow(scope);
            }}
            sx={{mt: 2}}
          >
            {t('settings:csp.addDirective', 'Add directive')}
          </Button>
        )}

        <Divider sx={{my: 2}} />
        <CopyableField
          label={t('settings:csp.header.label', 'Header sent for {{scope}}', {scope: scopeLabel(scope)})}
          value={buildCspHeader(rows, csp.reportOnly, data?.merged.reportUri)}
        />
        <Typography variant="caption" color="text.secondary">
          {t(
            'settings:csp.header.nonceHint',
            "Each response also adds a one-time nonce to script-src and style-src-elem, unless they allow 'unsafe-inline'.",
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
        <SettingsCard
          title={t('settings:csp.card.title', 'Directives')}
          description={t(
            'settings:csp.card.description',
            'The directives sent to the Console and Gate. Changing a directive overrides its default value.',
          )}
        >
          {body}
        </SettingsCard>
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
