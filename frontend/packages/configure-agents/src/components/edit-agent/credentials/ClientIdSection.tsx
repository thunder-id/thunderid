// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {CopyableField} from '@thunderid/configure-applications';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {OAuthAgentConfig} from '../../../models/agent';

interface ClientIdSectionProps {
  oauth2Config?: OAuthAgentConfig;
}

export default function ClientIdSection({oauth2Config = undefined}: ClientIdSectionProps): JSX.Element | null {
  const {t} = useTranslation();

  if (!oauth2Config?.clientId) return null;

  const clientIdLabel = t('agents:edit.credentials.sections.identifier.clientIdLabel', 'Client ID');
  const copyLabel = t('common:actions.copy');

  return (
    <SettingsCard
      title={t('agents:edit.credentials.sections.identifier.title', 'Identifier')}
      description={t(
        'agents:edit.credentials.sections.identifier.description',
        'Unique identifier used to reference this agent.',
      )}
    >
      <CopyableField
        id="agent-credentials-client-id"
        label={clientIdLabel}
        value={oauth2Config.clientId}
        copyAriaLabel={`${copyLabel} ${clientIdLabel}`}
        hint={t(
          'agents:edit.credentials.sections.identifier.clientIdHint',
          'The public OAuth2 client identifier this agent uses to authenticate as a client.',
        )}
      />
    </SettingsCard>
  );
}
