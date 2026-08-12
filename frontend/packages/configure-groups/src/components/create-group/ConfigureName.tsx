// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {Typography, Stack, TextField, FormControl, FormLabel} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import GroupConstraints from '../../constants/group-constraints';

/**
 * Props for the {@link ConfigureName} component.
 *
 * @public
 */
export interface ConfigureNameProps {
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
 * Step 1 of the group creation wizard: configure the group name.
 *
 * @public
 */
export default function ConfigureName({
  name,
  onNameChange,
  onReadyChange = undefined,
  hasMultipleOUs = false,
  organizationUnitName = undefined,
  organizationUnitLogoUrl = undefined,
  isOrganizationUnitLoading = false,
  onChangeOu = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();

  const trimmedLength = name.trim().length;

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(
        trimmedLength >= GroupConstraints.NAME_MIN_LENGTH && trimmedLength <= GroupConstraints.NAME_MAX_LENGTH,
      );
    }
  }, [trimmedLength, onReadyChange]);

  // An empty field is not an error yet; the user has simply not filled it in.
  const nameError = useMemo((): string | null => {
    if (trimmedLength === 0) {
      return null;
    }
    if (trimmedLength > GroupConstraints.NAME_MAX_LENGTH) {
      return t('groups:create.form.name.maxLength', {
        max: GroupConstraints.NAME_MAX_LENGTH,
        defaultValue: `Group name cannot exceed ${GroupConstraints.NAME_MAX_LENGTH} characters`,
      });
    }
    return null;
  }, [trimmedLength, t]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-name">
      <Typography variant="h1" gutterBottom>
        {t('groups:createWizard.name.title')}
      </Typography>

      {hasMultipleOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('groups:createWizard.organizationUnit.fieldLabel', 'Organization Unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="group-name-input">{t('groups:create.form.name.label')}</FormLabel>
        <TextField
          fullWidth
          id="group-name-input"
          value={name}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onNameChange(e.target.value)}
          placeholder={t('groups:create.form.name.placeholder')}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
          inputProps={{
            'data-testid': 'group-name-input',
          }}
        />

        <NameSuggestion onSelect={onNameChange} />
      </FormControl>
    </Stack>
  );
}
