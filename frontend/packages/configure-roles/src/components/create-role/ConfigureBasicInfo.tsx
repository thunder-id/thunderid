// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {Typography, Stack, TextField, FormControl, FormLabel} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import RoleConstraints from '../../constants/role-constraints';

export interface ConfigureBasicInfoProps {
  name: string;
  onNameChange: (name: string) => void;
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

/**
 * Step 1 of the role creation wizard: configure basic info (name + description).
 */
export default function ConfigureBasicInfo({
  name,
  onNameChange,
  onReadyChange = undefined,
  hasMultipleOUs = false,
  organizationUnitName = undefined,
  organizationUnitLogoUrl = undefined,
  isOrganizationUnitLoading = false,
  onChangeOu = undefined,
}: ConfigureBasicInfoProps): JSX.Element {
  const {t} = useTranslation();

  const trimmedLength = name.trim().length;

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(
        trimmedLength >= RoleConstraints.NAME_MIN_LENGTH && trimmedLength <= RoleConstraints.NAME_MAX_LENGTH,
      );
    }
  }, [trimmedLength, onReadyChange]);

  // An empty field is not an error yet; the user has simply not filled it in.
  const nameError = useMemo((): string | null => {
    if (trimmedLength === 0) {
      return null;
    }
    if (trimmedLength > RoleConstraints.NAME_MAX_LENGTH) {
      return t('roles:create.form.name.maxLength', {
        max: RoleConstraints.NAME_MAX_LENGTH,
        defaultValue: `Role name cannot exceed ${RoleConstraints.NAME_MAX_LENGTH} characters`,
      });
    }
    return null;
  }, [trimmedLength, t]);

  return (
    <Stack direction="column" spacing={4}>
      <Typography variant="h1" gutterBottom>
        {t('roles:createWizard.basicInfo.title')}
      </Typography>

      {hasMultipleOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('roles:createWizard.organizationUnit.fieldLabel', 'Organization Unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="role-name-input">{t('roles:create.form.name.label')}</FormLabel>
        <TextField
          fullWidth
          id="role-name-input"
          value={name}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onNameChange(e.target.value)}
          placeholder={t('roles:create.form.name.placeholder')}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
        />

        <NameSuggestion onSelect={onNameChange} />
      </FormControl>
    </Stack>
  );
}
