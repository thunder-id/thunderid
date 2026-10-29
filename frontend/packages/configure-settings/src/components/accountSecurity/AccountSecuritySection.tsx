// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Skeleton, Stack, Typography} from '@wso2/oxygen-ui';
import {InfoIcon} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import AccountLockCard from './AccountLockCard';
import AccountSuspensionSection from './AccountSuspensionSection';
import useGetAccountAccessConfig from '../../api/useGetAccountAccessConfig';
import useUpdateAccountAccessConfig from '../../api/useUpdateAccountAccessConfig';
import useAccountLockDraft from '../../hooks/useAccountLockDraft';
import {AccountLockCategories} from '../../models/accountAccess';

/**
 * The Account security tab: *Automatic account lock*, then *Lock and suspension flows*, so the card that
 * says what a lock does not do sits right above the one that does it.
 *
 * **The lock card edits the `accountAccess` section** through one draft, validation pass and save
 * bar for both categories. A save writes only what changed, so section fields this tab does not
 * show, such as sign-in activity, keep their saved values.
 *
 * **Suspension is read-only here.** It runs through administration flows started from each record's
 * page; the card lists those flows so an operator looking for it finds it.
 *
 * **A declarative `accountAccess` locks the section.** The server refuses every write while one
 * exists, so the lock card is read-only, there is no save, and a hint says where the settings live.
 *
 * Applications have no lock card. They resolve only at the application plane, where an
 * authentication hold is never enforced, so an application can never reach the locked state.
 *
 * @returns The rendered tab body
 *
 * @public
 */
export default function AccountSecuritySection(): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetAccountAccessConfig();
  const updateConfig = useUpdateAccountAccessConfig();
  const draft = useAccountLockDraft(data);
  const declarative = Boolean(data?.readOnly.user ?? data?.readOnly.agent);
  // Resolves an error through the `settings` catalog, per getErrorMessage's namespace contract, with
  // backend codes read from this section's own `accountLock.errors.*` keys.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => {
      if (key.includes(':')) {
        return t(key, options);
      }
      return t(key.startsWith('errors.') ? `settings:accountLock.${key}` : `settings:${key}`, options);
    },
    [t],
  );

  const clearSaveError = (): void => {
    if (updateConfig.isError) {
      updateConfig.reset();
    }
  };

  const handleSave = (): void => {
    if (!draft.validateAll()) {
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

  const pending = (): JSX.Element | undefined => {
    if (isLoading) {
      return (
        <Stack spacing={2}>
          {Array.from({length: AccountLockCategories.length}, (_, index) => (
            <Skeleton key={index} variant="rounded" height={120} />
          ))}
        </Stack>
      );
    }
    if (error) {
      return (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="settings:accountLock.load.error"
          fallbackDefaultValue="Failed to load the account security settings."
          onRetry={() => void refetch()}
        />
      );
    }
    return undefined;
  };

  const readOnlyHint: JSX.Element | null = declarative ? (
    <Stack direction="row" spacing={1} alignItems="flex-start" data-testid="account-lock-read-only-hint">
      <Box aria-hidden sx={{flex: 'none', display: 'inline-flex', mt: '2px', color: 'text.secondary'}}>
        <InfoIcon size={16} />
      </Box>
      <Typography variant="body2" color="text.secondary">
        {t(
          'settings:accountLock.readOnlyHint',
          "These settings are read-only because they're managed declaratively. Change them in the declarative configuration.",
        )}
      </Typography>
    </Stack>
  ) : null;

  return (
    <>
      <Stack spacing={3}>
        <SettingsCard
          title={t('settings:accountLock.section.title', 'Automatic account lock')}
          description={t(
            'settings:accountLock.section.description',
            "Temporarily block sign-in after too many failed attempts. Lockouts prevent new sign-ins but don't affect existing sessions or tokens. To revoke existing access, suspend the account.",
          )}
        >
          {pending() ?? (
            <Stack spacing={2}>
              {AccountLockCategories.map((category) => (
                <AccountLockCard
                  key={category}
                  category={category}
                  form={draft.forms[category]}
                  errors={draft.errors[category]}
                  disabled={declarative}
                  onChange={(patch) => {
                    clearSaveError();
                    draft.change(category, patch);
                  }}
                />
              ))}
              {readOnlyHint}
            </Stack>
          )}
        </SettingsCard>

        <AccountSuspensionSection />
      </Stack>

      {draft.dirty && !declarative && (
        <UnsavedChangesBar
          message={t('settings:accountLock.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('settings:accountLock.reset', 'Reset')}
          saveLabel={t('settings:accountLock.save', 'Save changes')}
          savingLabel={t('settings:accountLock.saving', 'Saving...')}
          isSaving={updateConfig.isPending}
          saveDisabled={draft.hasErrors}
          error={
            updateConfig.error
              ? getErrorMessage(
                  updateConfig.error,
                  tForErrors,
                  'accountLock.save.error',
                  'Failed to update the account security settings.',
                )
              : undefined
          }
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
