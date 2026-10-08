// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {generateHandle} from '@thunderid/utils';
import {Stack, TextField, FormControl, FormLabel, Typography} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect} from 'react';
import {useTranslation} from 'react-i18next';

export interface ConfigureNameProps {
  name: string;
  handle: string;
  handleEdited: boolean;
  onNameChange: (name: string) => void;
  onHandleChange: (handle: string) => void;
  onHandleEditedChange: (edited: boolean) => void;
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Whether the wizard's organization unit was picked on a dedicated earlier step (only then is
   * the summary chip shown).
   */
  hasMultipleOUs?: boolean;

  /**
   * The resolved organization unit's display name, shown in the summary chip.
   */
  organizationUnitName?: string;

  /**
   * The resolved organization unit's logo, shown in the summary chip.
   */
  organizationUnitLogoUrl?: string;

  /**
   * Whether the organization unit is still being resolved.
   */
  isOrganizationUnitLoading?: boolean;

  /**
   * Invoked when the chip's "Change" link is clicked, returning to the organization unit step.
   */
  onChangeOu?: () => void;
}

/** Create-wizard step 1: the name and its auto-derived (but editable) handle. */
export default function ConfigureName({
  name,
  handle,
  handleEdited,
  onNameChange,
  onHandleChange,
  onHandleEditedChange,
  onReadyChange = undefined,
  hasMultipleOUs = false,
  organizationUnitName = undefined,
  organizationUnitLogoUrl = undefined,
  isOrganizationUnitLoading = false,
  onChangeOu = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(name.trim().length > 0 && handle.trim().length > 0);
    }
  }, [name, handle, onReadyChange]);

  const handleNameChange = (e: ChangeEvent<HTMLInputElement>): void => {
    const newName = e.target.value;
    onNameChange(newName);
    if (!handleEdited) {
      onHandleChange(generateHandle(newName));
    }
  };

  const handleSuggestionSelect = (suggestion: string): void => {
    onNameChange(suggestion);
    onHandleChange(generateHandle(suggestion));
    onHandleEditedChange(false);
  };

  const handleHandleChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onHandleEditedChange(true);
    onHandleChange(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''));
  };

  return (
    <Stack direction="column" spacing={4} data-testid="configure-name">
      <Typography variant="h1" gutterBottom>
        {t('createWizard.name.title', "Let's collect some details about your verifiable presentation")}
      </Typography>

      {hasMultipleOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('create.organizationUnit.fieldLabel', 'Organization Unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="vp-name-input">{t('form.name.label')}</FormLabel>
        <TextField
          fullWidth
          id="vp-name-input"
          value={name}
          onChange={handleNameChange}
          placeholder={t('form.name.placeholder')}
          helperText={t('form.name.hint')}
        />

        <NameSuggestion onSelect={handleSuggestionSelect} />
      </FormControl>

      <FormControl fullWidth required>
        <FormLabel htmlFor="vp-handle-input">{t('form.handle.label')}</FormLabel>
        <TextField
          fullWidth
          id="vp-handle-input"
          value={handle}
          onChange={handleHandleChange}
          placeholder="eudi-pid"
          helperText={t('form.handle.hint')}
        />
      </FormControl>
    </Stack>
  );
}
