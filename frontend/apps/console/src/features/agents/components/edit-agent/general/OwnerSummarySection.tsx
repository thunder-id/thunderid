/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {SettingsCard} from '@thunderid/components';
import {useGetUsers} from '@thunderid/configure-users';
import {Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent} from '../../../models/agent';

interface OwnerSummarySectionProps {
  agent: Agent;
}

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs.username === 'string' ? attrs.username : undefined;
  const email = typeof attrs.email === 'string' ? attrs.email : undefined;
  return username ?? email ?? user.id;
};

/**
 * Read-only preview of this agent's owner, with no edit affordance — used on the General tab.
 * The Advanced tab is where the owner is actually assigned.
 */
export default function OwnerSummarySection({agent}: OwnerSummarySectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data: usersData} = useGetUsers({limit: 100, offset: 0});

  const ownerLabel = useMemo(() => {
    if (!agent.owner) return undefined;
    const match = (usersData?.users ?? []).find((user) => user.id === agent.owner);
    return match ? formatUserLabel(match) : agent.owner;
  }, [usersData, agent.owner]);

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.owner.title', 'Owner')}
      description={t(
        'agents:edit.general.sections.owner.summaryDescription',
        'The user who is accountable for this agent, shown in audit records and used as the contact point for questions about what this agent does. Assigning an owner does not give that user any special access to the agent. Manage this from the Advanced tab.',
      )}
    >
      {ownerLabel ? (
        <Typography variant="body1">{ownerLabel}</Typography>
      ) : (
        <Typography variant="body2" color="text.secondary">
          {t('agents:edit.general.owner.empty', 'No owner assigned')}
        </Typography>
      )}
    </SettingsCard>
  );
}
