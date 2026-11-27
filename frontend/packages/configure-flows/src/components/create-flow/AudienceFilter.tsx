// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ToggleButton, ToggleButtonGroup} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {TemplateAudiences} from '../../models/templates';

interface AudienceFilterProps {
  value: TemplateAudiences;
  onChange: (audience: TemplateAudiences) => void;
}

/**
 * Narrows the Create Flow wizard to the flows written for people, for AI agents, or for both. The same filter sits on
 * the flow type step and the template step.
 */
export default function AudienceFilter({value, onChange}: AudienceFilterProps): JSX.Element {
  const {t} = useTranslation();

  const choices: {audience: TemplateAudiences; label: string}[] = [
    {audience: TemplateAudiences.All, label: t('flows:create.audience.all', 'All')},
    {audience: TemplateAudiences.User, label: t('flows:create.audience.user', 'Users')},
    {audience: TemplateAudiences.Agent, label: t('flows:create.audience.agent', 'Agents')},
  ];

  return (
    <ToggleButtonGroup
      exclusive
      size="small"
      value={value}
      aria-label={t('flows:create.audience.label', 'Filter by audience')}
      onChange={(_event, audience: TemplateAudiences | null) => {
        if (audience) onChange(audience);
      }}
    >
      {choices.map((choice) => (
        <ToggleButton key={choice.audience} value={choice.audience} sx={{textTransform: 'none'}}>
          {choice.label}
        </ToggleButton>
      ))}
    </ToggleButtonGroup>
  );
}
