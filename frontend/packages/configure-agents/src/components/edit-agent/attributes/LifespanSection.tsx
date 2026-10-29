// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormLabel} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useLifespanEndText from '../../../hooks/useLifespanEndText';
import type {Agent} from '../../../models/agent';
import ValidUntilPicker from '../../ValidUntilPicker';

interface LifespanSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onValidationChange?: (hasError: boolean) => void;
}

/**
 * When the agent's lifespan ends. A new end must be in the future, but an expired agent's stored end is
 * shown as it is, so the section does not block saving unrelated changes.
 */
export default function LifespanSection({
  agent,
  editedAgent,
  onFieldChange,
  onValidationChange = undefined,
}: LifespanSectionProps): JSX.Element {
  const {t} = useTranslation();
  const lifespanEndText = useLifespanEndText();
  const [hasError, setHasError] = useState(false);

  const isEdited = editedAgent.validUntil !== undefined;
  const validUntil = isEdited ? editedAgent.validUntil : agent.validUntil;

  const handleErrorChange = (error: boolean): void => {
    setHasError(error);
    onValidationChange?.(error);
  };

  let helperText: string;
  if (hasError) {
    helperText = t('agents:edit.attributes.lifespan.mustBeFuture', 'Choose a date and time in the future.');
  } else {
    helperText =
      lifespanEndText(validUntil) ??
      t('agents:edit.attributes.lifespan.unbounded', 'No expiry date. Set one to end its access automatically.');
  }

  return (
    <SettingsCard
      title={t('agents:edit.attributes.lifespan.title', 'Lifespan')}
      description={t(
        'agents:edit.attributes.lifespan.description',
        'When this agent stops being able to authenticate. Tokens issued to it expire no later than this.',
      )}
    >
      <FormControl fullWidth size="small">
        <FormLabel htmlFor="agent-valid-until">{t('agents:edit.attributes.lifespan.label', 'Valid Until')}</FormLabel>
        <ValidUntilPicker
          id="agent-valid-until"
          value={validUntil}
          // Only a newly chosen end must be in the future; the stored one is shown even once it has passed.
          disablePast={isEdited}
          disabled={agent.isReadOnly}
          helperText={helperText}
          onErrorChange={handleErrorChange}
          onChange={(value) => onFieldChange('validUntil', value)}
        />
      </FormControl>
    </SettingsCard>
  );
}
