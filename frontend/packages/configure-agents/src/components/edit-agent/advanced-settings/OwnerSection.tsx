// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useGetAdministrationFlowHandle, useGetUsers} from '@thunderid/configure-users';
import {AdministrationModes, useAdministrationOperation} from '@thunderid/contexts';
import {AdministrationFlowConfigKey} from '@thunderid/utils';
import {Box, MenuItem, Select, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import OwnerVerificationPanel from './OwnerVerificationPanel';
import {PERSONAL_AGENT_KIND, type Agent, type AgentOwner} from '../../../models/agent';
import {isVerifiableOwner} from '../../../utils/externalOwner';
import AgentOwnerField from '../../AgentOwnerField';
import OwnerVerificationStatus from '../../OwnerVerificationStatus';

interface OwnerSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  /** Whether the signed-in admin may change the agent here. */
  readOnly?: boolean;
}

/** Whether the edited owner differs from the saved one, so it would be lost by verifying the saved owner. */
const isOwnerUnsaved = (saved?: AgentOwner, edited?: AgentOwner): boolean =>
  edited !== undefined &&
  (edited.id !== saved?.id || edited.idType !== saved?.idType || edited.isLocal !== saved?.isLocal);

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

/**
 * Shows and edits the agent's owner. A read-only agent's owner, and a personal agent's, cannot be changed. An
 * external owner that can be verified shows whether it is, and where the deployment runs flows, a way to send it a
 * verification link. The link is sent through the flow the server's `adminAgentOwnerVerificationFlow` setting names;
 * with none set, for an agent shared into this organization unit, for an operator who may not read the server
 * configuration, or when it cannot be read, only the status is shown, and a failed read says so.
 */
export default function OwnerSection({
  agent,
  editedAgent,
  onFieldChange,
  readOnly = false,
}: OwnerSectionProps): JSX.Element {
  const {t} = useTranslation();

  const {data: usersData, isLoading: usersLoading} = useGetUsers({limit: 100, offset: 0});

  const options: UserOption[] = useMemo(
    () => (usersData?.users ?? []).map((user) => ({id: user.id, label: formatUserLabel(user)})),
    [usersData],
  );

  const owner = editedAgent.owner ?? agent.owner;
  // An owner managed in another identity system is not one of these users, so the picker shows none.
  const internalOwnerId = owner?.isLocal === false ? '' : (owner?.id ?? '');
  // A personal agent keeps the owner it was registered for.
  const isPersonal = agent.kind === PERSONAL_AGENT_KIND;
  const isOwnerLocked = readOnly || isPersonal;
  const ownerUnsaved = isOwnerUnsaved(agent.owner, editedAgent.owner);
  const verifiable =
    useAdministrationOperation('agents', 'verifyOwner') === AdministrationModes.FLOW && isVerifiableOwner(agent.owner);
  // The organization unit an agent is shared into cannot send its owner a link.
  const canSend = verifiable && agent.isOwnedByOU !== false;
  const {data: verificationHandle, error: handleError} = useGetAdministrationFlowHandle(
    AdministrationFlowConfigKey.ADMIN_AGENT_OWNER_VERIFICATION,
    canSend,
  );
  // An operator who may not read the server configuration is not offered the link, as for a user's contacts. Any
  // other failure to read it is reported.
  const handleUnreadable =
    handleError !== null &&
    handleError !== undefined &&
    (handleError as {response?: {status?: number}}).response?.status !== 403;
  let verification: JSX.Element | null = null;
  if (canSend && verificationHandle) {
    verification = (
      <OwnerVerificationPanel
        // A link sent to an earlier owner says nothing about this one.
        key={`${agent.owner?.idType}:${agent.owner?.id}`}
        agent={agent}
        ownerUnsaved={ownerUnsaved}
        handle={verificationHandle}
      />
    );
  } else if (verifiable && !ownerUnsaved) {
    verification = (
      <Box data-testid="agent-owner-verification-status" sx={{mt: 2}}>
        <OwnerVerificationStatus owner={agent.owner} />
        {canSend && handleUnreadable && (
          <Typography variant="body2" color="error" sx={{mt: 1}}>
            {t(
              'agents:owner.verification.settingUnreadable',
              'A verification link cannot be sent from here, because the server configuration could not be read.',
            )}
          </Typography>
        )}
      </Box>
    );
  }

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.owner.title', 'Owner')}
      description={
        isPersonal
          ? t(
              'agents:edit.general.sections.owner.personalDescription',
              'The user this personal agent acts for. It keeps the owner it was registered for.',
            )
          : t('agents:edit.general.sections.owner.description', 'The user accountable for this agent.')
      }
    >
      <AgentOwnerField
        id="agent-owner"
        size="small"
        initialOwner={agent.owner}
        disabled={isOwnerLocked}
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
            disabled={usersLoading || isOwnerLocked}
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
      {verification}
    </SettingsCard>
  );
}
