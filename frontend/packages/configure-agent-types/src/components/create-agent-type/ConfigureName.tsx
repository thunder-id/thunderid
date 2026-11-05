// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {UserTypeConstraints} from '@thunderid/configure-user-types';
import {generateHandle} from '@thunderid/utils';
import {Typography, Stack, TextField} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link ConfigureName} component.
 *
 * @public
 */
export interface ConfigureNameProps {
  /**
   * The agent type's display name.
   */
  name: string;
  /**
   * The agent type's handle.
   */
  handle: string;
  /**
   * Whether the handle was typed by hand, which stops it following the name.
   */
  handleEdited: boolean;
  /**
   * Invoked when the name changes.
   */
  onNameChange: (name: string) => void;
  /**
   * Invoked when the handle changes.
   */
  onHandleChange: (handle: string) => void;
  /**
   * Invoked when the handle switches between suggested and typed by hand.
   */
  onHandleEditedChange: (edited: boolean) => void;
  /**
   * Invoked whenever the step becomes ready or stops being ready to continue.
   */
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
 * The details step of the agent type creation wizard: configure the agent type name, its handle,
 * and its organization unit (summarized, when picked on an earlier step).
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
      return t('agentTypes:createWizard.name.maxLength', {
        max: UserTypeConstraints.NAME_MAX_LENGTH,
        defaultValue: 'Agent type name cannot exceed {{max}} characters',
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
      return t('agentTypes:createWizard.handle.maxLength', {
        max: UserTypeConstraints.HANDLE_MAX_LENGTH,
        defaultValue: 'Handle cannot exceed {{max}} characters',
      });
    }
    return t(
      'agentTypes:createWizard.handle.invalid',
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
        {t('agentTypes:createWizard.name.title', "Let's collect some details about your agent type")}
      </Typography>

      {hasMultipleOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('agentTypes:createWizard.organizationUnit.fieldLabel', 'Organization unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <Stack>
        <TextField
          labelPlacement="top"
          label={t('agentTypes:createWizard.name.fieldLabel', 'Agent type name')}
          required
          fullWidth
          id="agent-type-name-input"
          value={name}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => handleNameChange(e.target.value)}
          placeholder={t('agentTypes:createWizard.name.placeholder', 'Enter your agent type name')}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
          inputProps={{
            'data-testid': 'agent-type-name-input',
          }}
        />

        <NameSuggestion onSelect={handleSuggestionSelect} />
      </Stack>

      <TextField
        labelPlacement="top"
        label={t('agentTypes:createWizard.handle.fieldLabel', 'Handle')}
        required
        fullWidth
        id="agent-type-handle-input"
        value={handle}
        onChange={handleHandleChange}
        placeholder={t('agentTypes:createWizard.handle.placeholder', 'e.g., assistant')}
        error={Boolean(handleError)}
        helperText={
          handleError ??
          t(
            'agentTypes:createWizard.handle.hint',
            "A unique identifier for this agent type. You can't change it once created.",
          )
        }
        inputProps={{
          'data-testid': 'agent-type-handle-input',
        }}
      />
    </Stack>
  );
}
