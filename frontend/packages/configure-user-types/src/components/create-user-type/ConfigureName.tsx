// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip, ToggleCard} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {generateHandle} from '@thunderid/utils';
import {Typography, Stack, TextField, FormControl, FormLabel} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import UserTypeConstraints from '../../constants/user-type-constraints';

/**
 * Props for the {@link ConfigureName} component.
 *
 * @public
 */
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

  /**
   * Whether self-registration is allowed for this user type.
   */
  allowSelfRegistration: boolean;

  /**
   * Invoked when the self-registration toggle changes.
   */
  onAllowSelfRegistrationChange: (allow: boolean) => void;
}

/**
 * Step 1 of the user type creation wizard: configure the user type name, its organization unit
 * (summarized, when picked on an earlier step), and whether it allows self-registration.
 *
 * @public
 */
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
  allowSelfRegistration,
  onAllowSelfRegistrationChange,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();

  const trimmedLength = name.trim().length;
  const trimmedHandle = handle.trim();

  const isHandleValid =
    trimmedHandle.length <= UserTypeConstraints.HANDLE_MAX_LENGTH &&
    UserTypeConstraints.HANDLE_PATTERN.test(trimmedHandle);

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(
        trimmedLength >= UserTypeConstraints.NAME_MIN_LENGTH &&
          trimmedLength <= UserTypeConstraints.NAME_MAX_LENGTH &&
          isHandleValid,
      );
    }
  }, [trimmedLength, isHandleValid, onReadyChange]);

  // An empty field is not an error yet; the user has simply not filled it in.
  const nameError = useMemo((): string | null => {
    if (trimmedLength > UserTypeConstraints.NAME_MAX_LENGTH) {
      return t('userTypes:createWizard.name.maxLength', {
        max: UserTypeConstraints.NAME_MAX_LENGTH,
        defaultValue: `User type name cannot exceed ${UserTypeConstraints.NAME_MAX_LENGTH} characters`,
      });
    }
    return null;
  }, [trimmedLength, t]);

  // An empty field is not an error yet; the user has simply not filled it in.
  const handleError = useMemo((): string | null => {
    if (trimmedHandle.length === 0 || isHandleValid) {
      return null;
    }
    if (trimmedHandle.length > UserTypeConstraints.HANDLE_MAX_LENGTH) {
      return t('userTypes:createWizard.handle.maxLength', {
        max: UserTypeConstraints.HANDLE_MAX_LENGTH,
        defaultValue: `Handle cannot exceed ${UserTypeConstraints.HANDLE_MAX_LENGTH} characters`,
      });
    }
    return t(
      'userTypes:createWizard.handle.invalid',
      'Handle must start and end with a lowercase letter or number, and may contain only lowercase letters, numbers, hyphens, and underscores',
    );
  }, [trimmedHandle, isHandleValid, t]);

  const handleNameChange = (newName: string): void => {
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
    onHandleChange(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''));
  };

  return (
    <Stack direction="column" spacing={4} data-testid="configure-name">
      <Typography variant="h1" gutterBottom>
        {t('userTypes:createWizard.name.title', "Let's collect some details about your user type")}
      </Typography>

      {hasMultipleOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('userTypes:createWizard.organizationUnit.fieldLabel', 'Organization Unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="user-type-name-input">{t('userTypes:createWizard.name.fieldLabel')}</FormLabel>
        <TextField
          fullWidth
          id="user-type-name-input"
          value={name}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => handleNameChange(e.target.value)}
          placeholder={t('userTypes:createWizard.name.placeholder')}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
          inputProps={{
            'data-testid': 'user-type-name-input',
          }}
        />

        <NameSuggestion onSelect={handleSuggestionSelect} />
      </FormControl>

      <FormControl fullWidth required>
        <FormLabel htmlFor="user-type-handle-input">
          {t('userTypes:createWizard.handle.fieldLabel', 'Handle')}
        </FormLabel>
        <TextField
          fullWidth
          id="user-type-handle-input"
          value={handle}
          onChange={handleHandleChange}
          placeholder={t('userTypes:createWizard.handle.placeholder', 'e.g., customer')}
          error={Boolean(handleError)}
          helperText={
            handleError ??
            t(
              'userTypes:createWizard.handle.hint',
              "A unique identifier for this user type. You can't change it once created.",
            )
          }
          inputProps={{
            'data-testid': 'user-type-handle-input',
          }}
        />
      </FormControl>

      <ToggleCard
        checked={allowSelfRegistration}
        onChange={onAllowSelfRegistrationChange}
        title={t('userTypes:allowSelfRegistration', 'Allow Self Registration')}
        subtitle={t(
          'userTypes:createWizard.general.subtitle',
          'Users can register for this user type without an invitation',
        )}
      />
    </Stack>
  );
}
