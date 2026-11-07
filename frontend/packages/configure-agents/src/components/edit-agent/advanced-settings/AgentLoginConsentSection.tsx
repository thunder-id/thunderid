// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {LoginConsentSection, hasUserAccess} from '@thunderid/configure-applications';
import type {JSX} from 'react';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

interface AgentLoginConsentSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  /** Reports whether the card holds a validity period it could not commit, to gate Save. */
  onValidationChange?: (hasError: boolean) => void;
}

export default function AgentLoginConsentSection({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
}: AgentLoginConsentSectionProps): JSX.Element | null {
  // Consent is collected only in flows that sign a user in, such as authorization code and CIBA, so
  // the validity period only matters when the agent holds one of those grants.
  const isApplicable = Boolean(oauth2Config && hasUserAccess(oauth2Config.grantTypes));

  if (!isApplicable) return null;

  return (
    <LoginConsentSection
      loginConsent={editedAgent.loginConsent ?? agent.loginConsent}
      onLoginConsentChange={
        agent.isReadOnly ? undefined : (loginConsent) => onFieldChange('loginConsent', loginConsent)
      }
      disabled={agent.isReadOnly}
      onValidationChange={onValidationChange}
    />
  );
}
