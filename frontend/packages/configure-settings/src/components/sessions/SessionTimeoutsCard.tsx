// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Divider, Skeleton, Stack} from '@wso2/oxygen-ui';
import {useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetServerConfigSection from '../../api/useGetServerConfigSection';
import useUpdateServerConfigSection from '../../api/useUpdateServerConfigSection';
import SettingsQueryKeys from '../../constants/settings-query-keys';
import useSessionTimeoutsDraft, {isSessionDeclarative} from '../../hooks/useSessionTimeoutsDraft';
import type {SessionTimeoutField, SessionValue} from '../../models/session';
import DurationField from '../common/DurationField';
import SectionLockNotice from '../common/SectionLockNotice';
import SettingField from '../common/SettingField';

/**
 * The *Session timeouts* card: how long a sign-in session lasts without activity and in all, as the
 * `session` section sets them.
 *
 * Each timeout shows the value in effect and whether it is the default or set here, and a value set
 * here can be reset to the default. A save sends the saved values with the draft's changes.
 *
 * The card is read-only, with no save, when a declarative resource sets a timeout, where it names the
 * file the section is changed in instead, and for an admin who may not update server configuration,
 * which covers a data plane.
 *
 * @returns The rendered card
 *
 * @public
 */
export default function SessionTimeoutsCard(): JSX.Element {
  const {t, i18n} = useTranslation();
  const {data, error, refetch} = useGetServerConfigSection<SessionValue>(SettingsQueryKeys.SESSION);
  const updateConfig = useUpdateServerConfigSection<SessionValue>(
    SettingsQueryKeys.SESSION,
    t('settings:session.save.success', 'Session settings updated.'),
  );
  const draft = useSessionTimeoutsDraft(data);
  const {can} = useCapabilitiesContext();
  const declarative: boolean = isSessionDeclarative(data);
  const readOnly: boolean = declarative || !can('serverconfig', 'update');
  // A save's success resets the draft, so nothing may change while one is in flight.
  const disabled: boolean = readOnly || updateConfig.isPending;

  // Resolves an error through the `settings` catalog, per getErrorMessage's namespace contract: an
  // explicit `ns:` key is forwarded unchanged, and a bare key resolves to this section's own wording
  // when there is one, else to the wording every server-config section shares. Every lookup forwards
  // the caller's options, so the caller's default applies when neither has the key: the empty one
  // getErrorMessage passes for a code, which sends it on to the `common` catalog and its fallback.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => {
      if (key.includes(':')) {
        return t(key, options);
      }
      const sectionKey = `settings:session.${key}`;
      const sharedKey = `settings:serverConfig.${key}`;
      return t(!i18n.exists(sectionKey) && i18n.exists(sharedKey) ? sharedKey : sectionKey, options);
    },
    [i18n, t],
  );

  const fields: {field: SessionTimeoutField; label: string; hint: string; testId: string}[] = [
    {
      field: 'idleTimeoutSeconds',
      label: t('settings:session.idleTimeout.label', 'Idle timeout'),
      hint: t('settings:session.idleTimeout.hint', 'Signs the user out after this long without activity.'),
      testId: 'session-idle-timeout',
    },
    {
      field: 'absoluteTimeoutSeconds',
      label: t('settings:session.absoluteTimeout.label', 'Absolute timeout'),
      hint: t(
        'settings:session.absoluteTimeout.hint',
        "Signs the user out this long after they sign in, even if they're active.",
      ),
      testId: 'session-absolute-timeout',
    },
  ];

  // A previous save error is out of date once the draft changes again.
  const clearSaveError = (): void => {
    if (updateConfig.isError) {
      updateConfig.reset();
    }
  };

  const handleSave = (): void => {
    if (draft.hasErrors) {
      return;
    }
    updateConfig.mutate(
      {data: draft.buildPayload()},
      {
        onSuccess: () => {
          draft.reset();
        },
      },
    );
  };

  const saveError: string | undefined = updateConfig.error
    ? getErrorMessage(updateConfig.error, tForErrors, 'save.error', 'Failed to update the session settings.')
    : undefined;

  let body: JSX.Element;
  if (data) {
    body = (
      <Stack spacing={3}>
        {/* A later read that failed leaves the values read before in place, and says so. */}
        {error && (
          <QueryErrorNotice
            error={error}
            t={tForErrors}
            variant="inline"
            fallbackKey="settings:session.refresh.error"
            fallbackDefaultValue="Failed to refresh the session settings. The values shown are the ones loaded before."
            onRetry={() => void refetch()}
          />
        )}
        {declarative && <SectionLockNotice section={SettingsQueryKeys.SESSION} />}
        <Stack spacing={3} divider={<Divider />}>
          {fields.map(({field, label, hint, testId}) => (
            <SettingField
              key={field}
              label={label}
              hint={hint}
              custom={draft.custom[field]}
              disabled={disabled}
              onReset={
                readOnly
                  ? undefined
                  : () => {
                      clearSaveError();
                      draft.resetToDefault(field);
                    }
              }
            >
              {(hintId) => (
                <DurationField
                  inline
                  label={label}
                  seconds={draft.values[field]}
                  error={draft.errors[field]}
                  disabled={disabled}
                  describedBy={hintId}
                  testId={testId}
                  onChange={(seconds) => {
                    clearSaveError();
                    draft.change(field, seconds);
                  }}
                />
              )}
            </SettingField>
          ))}
        </Stack>
      </Stack>
    );
  } else if (error) {
    body = (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="inline"
        fallbackKey="settings:session.load.error"
        fallbackDefaultValue="Failed to load the session settings."
        onRetry={() => void refetch()}
      />
    );
  } else {
    body = (
      <Stack spacing={2}>
        {fields.map(({field}) => (
          <Skeleton key={field} variant="rounded" height={72} />
        ))}
      </Stack>
    );
  }

  return (
    <>
      <SettingsCard
        title={t('settings:session.card.title', 'Session timeouts')}
        description={t(
          'settings:session.card.description',
          'How long a sign-in session lasts. A change applies without a restart, and a shorter timeout also ends existing sessions that have gone past it.',
        )}
      >
        {body}
      </SettingsCard>

      {draft.dirty && !readOnly && (
        <UnsavedChangesBar
          message={t('settings:saveBar.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('settings:saveBar.save', 'Save changes')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={updateConfig.isPending}
          saveDisabled={draft.hasErrors}
          resetDisabled={updateConfig.isPending}
          error={saveError}
          onReset={() => {
            clearSaveError();
            draft.reset();
          }}
          onSave={handleSave}
        />
      )}
    </>
  );
}
