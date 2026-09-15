// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import type {InboundAuthConfig} from '@thunderid/configure-applications';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ClientAccessTokenSection from '../../../../applications/components/edit-application/token-settings/ClientAccessTokenSection';
import TokenConstants from '../../../../applications/constants/token-constants';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

interface AgentAccessTokenSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Access-token settings for the agent acting as its own client (client_credentials grant), backed
 * by the shared ClientAccessTokenSection. The selectable claims merge the agent's type-schema
 * attributes with the agent system, OU, group, and role attributes.
 */
export default function AgentAccessTokenSection({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
}: AgentAccessTokenSectionProps): JSX.Element {
  const {t} = useTranslation();

  const {data: agentTypesData} = useGetAgentTypes();
  const matchedSchema = agentTypesData?.types?.find((s) => s.name === agent.type);
  const {data: schemaDetails, isLoading} = useGetAgentType(matchedSchema?.id);

  const schemaAttributes = schemaDetails?.schema
    ? Object.entries(schemaDetails.schema)
        .filter(([, fieldDef]) => !((fieldDef.type === 'string' || fieldDef.type === 'number') && fieldDef.credential))
        .map(([name]) => name)
    : [];

  const availableAttributes = Array.from(
    new Set([...schemaAttributes, ...TokenConstants.ADDITIONAL_AGENT_ATTRIBUTES]),
  ).sort();

  const inboundAuthConfig = (editedAgent.inboundAuthConfig ??
    agent.inboundAuthConfig ??
    []) as unknown as InboundAuthConfig[];

  return (
    <ClientAccessTokenSection
      oauth2Config={oauth2Config}
      inboundAuthConfig={inboundAuthConfig}
      onFieldChange={onFieldChange}
      availableAttributes={availableAttributes}
      isLoadingAttributes={isLoading}
      disabled={agent.isReadOnly === true}
      onValidationChange={onValidationChange}
      inputId="agent-access-token-validity"
      subjectValue={agent.id}
      subjectType="agent"
      copy={{
        attributesTitle: t('agents:edit.tokens.agent.attributes.title', 'Access Token Attributes'),
        attributesDescription: t(
          'agents:edit.tokens.agent.attributes.description',
          'Extra attributes to add to the access token this agent receives for itself.',
        ),
        attributesLabel: t('agents:edit.tokens.agent.attributes.label', 'Add or Remove Attributes'),
        attributesHint: t(
          'agents:edit.tokens.agent.attributes.hint',
          'Click on agent attributes to add them to its access token.',
        ),
        attributesEmpty: t(
          'agents:edit.tokens.agent.attributes.empty',
          'No attributes available. Configure attributes for this agent in the Attributes tab.',
        ),
        validityTitle: t('agents:edit.tokens.agent.validity.title', 'Token Validity'),
        validityDescription: t(
          'agents:edit.tokens.agent.validity.description',
          'How long this access token remains valid before expiration.',
        ),
        validityLabel: t('agents:edit.tokens.agent.validity.label', 'Token Validity'),
        validityHint: t(
          'agents:edit.tokens.agent.validity.hint',
          'Token validity period in seconds (e.g., 3600 for 1 hour).',
        ),
        validityError: t('agents:edit.tokens.agent.validity.error', 'Enter a validity period of at least 1 second.'),
      }}
    />
  );
}
