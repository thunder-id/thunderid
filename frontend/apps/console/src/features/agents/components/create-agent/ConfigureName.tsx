// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, OrganizationUnitSummaryChip} from '@thunderid/components';
import {OrganizationUnitTreeConstants} from '@thunderid/configure-organization-units';
import {FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AgentConstants from '../../constants/agent-constants';

export interface ConfigureNameProps {
  agentName: string;
  onAgentNameChange: (name: string) => void;
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Whether the wizard's organization unit was picked on a dedicated earlier step (only then is
   * the summary chip shown).
   */
  hasChildOUs?: boolean;

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

export default function ConfigureName({
  agentName,
  onAgentNameChange,
  onReadyChange = undefined,
  hasChildOUs = false,
  organizationUnitName = undefined,
  organizationUnitLogoUrl = undefined,
  isOrganizationUnitLoading = false,
  onChangeOu = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();

  const trimmedLength = agentName.trim().length;

  useEffect((): void => {
    onReadyChange?.(trimmedLength >= AgentConstants.NAME_MIN_LENGTH && trimmedLength <= AgentConstants.NAME_MAX_LENGTH);
  }, [trimmedLength, onReadyChange]);

  // An empty field is not an error yet; the user has simply not filled it in.
  const nameError = useMemo((): string | null => {
    if (trimmedLength > AgentConstants.NAME_MAX_LENGTH) {
      return t('agents:createWizard.name.maxLength', {
        max: AgentConstants.NAME_MAX_LENGTH,
        defaultValue: `Agent name cannot exceed ${AgentConstants.NAME_MAX_LENGTH} characters`,
      });
    }
    return null;
  }, [trimmedLength, t]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-agent-name">
      <Typography variant="h1" gutterBottom>
        {t('agents:createWizard.name.title', "Let's collect some details about your agent")}
      </Typography>

      {hasChildOUs && onChangeOu && (
        <OrganizationUnitSummaryChip
          logoUrl={organizationUnitLogoUrl}
          icon={OrganizationUnitTreeConstants.DEFAULT_AVATAR}
          label={t('agents:createWizard.organizationUnit.fieldLabel', 'Organization Unit')}
          value={isOrganizationUnitLoading ? t('common:status.loading', 'Loading...') : organizationUnitName}
          onChange={onChangeOu}
        />
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="agent-name-input">{t('agents:createWizard.name.fieldLabel', 'Agent name')}</FormLabel>
        <TextField
          fullWidth
          id="agent-name-input"
          value={agentName}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onAgentNameChange(e.target.value)}
          placeholder={t('agents:createWizard.name.placeholder', 'e.g. Billing Service')}
          error={Boolean(nameError)}
          helperText={nameError ?? undefined}
          inputProps={{'data-testid': 'agent-name-input'}}
        />

        <NameSuggestion onSelect={onAgentNameChange} />
      </FormControl>
    </Stack>
  );
}
