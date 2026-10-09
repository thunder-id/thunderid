// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion} from '@thunderid/components';
import {generateHandle} from '@thunderid/utils';
import {FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {NotificationChannel} from '../../models/notification-template';

const HANDLE_PATTERN = /^[a-z0-9][a-z0-9_-]*[a-z0-9]$|^[a-z0-9]$/;

/**
 * Props for the {@link ConfigureTemplateIdentity} component.
 *
 * @public
 */
export interface ConfigureTemplateIdentityProps {
  channel: NotificationChannel;
  displayName: string;
  handle: string;
  /** True once the user edits the handle directly, so it stops auto-deriving from the name. */
  handleEdited: boolean;
  onDisplayNameChange: (value: string) => void;
  onHandleChange: (value: string) => void;
  onHandleEditedChange: (edited: boolean) => void;
  /** Broadcasts whether these fields are valid. */
  onReadyChange: (ready: boolean) => void;
  /** Existing handles in the channel, to flag a duplicate before submit. */
  existingHandles?: string[];
}

/**
 * The first creation step: the template's identity — display name (with a suggestion) and handle
 * (auto-derived from the name until edited).
 *
 * @public
 */
export default function ConfigureTemplateIdentity({
  channel,
  displayName,
  handle,
  handleEdited,
  onDisplayNameChange,
  onHandleChange,
  onHandleEditedChange,
  onReadyChange,
  existingHandles = [],
}: ConfigureTemplateIdentityProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');

  const handleNameChange = (name: string): void => {
    onDisplayNameChange(name);
    if (!handleEdited) onHandleChange(generateHandle(name));
  };

  const handleHandleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onHandleEditedChange(true);
    onHandleChange(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''));
  };

  const handleSuggestionSelect = (suggestion: string): void => {
    onHandleEditedChange(false);
    onDisplayNameChange(suggestion);
    onHandleChange(generateHandle(suggestion));
  };

  const isDuplicateHandle = handle !== '' && existingHandles.includes(handle);
  const handleError = isDuplicateHandle
    ? t('create.handle.duplicate', 'A template with this handle already exists.')
    : handle !== '' && !HANDLE_PATTERN.test(handle)
      ? t('create.handle.invalid', 'Use lowercase letters, numbers, hyphens and underscores.')
      : null;

  const ready = displayName.trim() !== '' && handle.trim() !== '' && !handleError;
  useEffect(() => {
    onReadyChange(ready);
  }, [ready, onReadyChange]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-template-identity">
      <Typography variant="h1" gutterBottom>
        {channel === 'email'
          ? t('create.details.titleEmail', "Let's collect some details about your email template")
          : t('create.details.titleSms', "Let's collect some details about your SMS template")}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel htmlFor="template-name-input">{t('create.displayName.label', 'Display name')}</FormLabel>
        <TextField
          fullWidth
          id="template-name-input"
          value={displayName}
          onChange={(e) => handleNameChange(e.target.value)}
          placeholder={t('create.displayName.placeholder', 'e.g. OTP Verification')}
          inputProps={{maxLength: 255}}
        />
        <NameSuggestion onSelect={handleSuggestionSelect} />
      </FormControl>

      <FormControl fullWidth required>
        <FormLabel htmlFor="template-handle-input">{t('create.handle.label', 'Handle')}</FormLabel>
        <TextField
          fullWidth
          id="template-handle-input"
          value={handle}
          onChange={handleHandleChange}
          placeholder={t('create.handle.placeholder', 'e.g. otp-verification')}
          error={Boolean(handleError)}
          helperText={
            handleError ?? t('create.handle.helperText', 'A unique, URL-friendly identifier. Cannot be changed later.')
          }
          inputProps={{maxLength: 255, sx: {fontFamily: 'monospace'}}}
        />
      </FormControl>
    </Stack>
  );
}
