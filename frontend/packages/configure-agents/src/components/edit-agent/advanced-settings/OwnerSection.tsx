// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useGetUsers} from '@thunderid/configure-users';
import {MenuItem, Select, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent} from '../../../models/agent';
import AgentOwnerField from '../../AgentOwnerField';

interface OwnerSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

interface UserOption {
  id: string;
  label: string;
}

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs['username'] === 'string' ? attrs['username'] : undefined;
  const email = typeof attrs['email'] === 'string' ? attrs['email'] : undefined;
  return username ?? email ?? user.id;
};

export default function OwnerSection({agent, editedAgent, onFieldChange}: OwnerSectionProps): JSX.Element {
  const {t} = useTranslation();

  const {data: usersData, isLoading: usersLoading} = useGetUsers({limit: 100, offset: 0, includeAll: true});

  const options: UserOption[] = useMemo(
    () => (usersData?.users ?? []).map((user) => ({id: user.id, label: formatUserLabel(user)})),
    [usersData],
  );

  const owner = editedAgent.owner ?? agent.owner;
  // An owner managed in another identity system is not one of these users, so the picker shows none.
  const internalOwnerId = owner?.isLocal === false ? '' : (owner?.id ?? '');

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.owner.title', 'Owner')}
      description={t('agents:edit.general.sections.owner.description', 'The user accountable for this agent.')}
    >
      <AgentOwnerField
        id="agent-owner"
        size="small"
        initialOwner={agent.owner}
        disabled={agent.isReadOnly}
        // Choosing Internal again leaves the saved owner in place until a user is picked.
        onExternalChange={(external) => onFieldChange('owner', external)}
        internalPicker={
          <Select
            id="agent-owner-select"
            labelId="agent-owner-label"
            size="small"
            fullWidth
            value={internalOwnerId}
            displayEmpty
            disabled={usersLoading || agent.isReadOnly}
            onChange={(e) =>
              onFieldChange(
                'owner',
                e.target.value ? {id: String(e.target.value), idType: 'uuid', isLocal: true} : undefined,
              )
            }
            renderValue={(selected) =>
              !selected ? (
                <Typography color="text.secondary" variant="body2">
                  {t('agents:edit.general.owner.empty', 'No owner assigned')}
                </Typography>
              ) : (
                (options.find((opt) => opt.id === selected)?.label ?? selected)
              )
            }
          >
            {options.map((opt) => (
              <MenuItem key={opt.id} value={opt.id}>
                {opt.label}
              </MenuItem>
            ))}
          </Select>
        }
      />
    </SettingsCard>
  );
}
