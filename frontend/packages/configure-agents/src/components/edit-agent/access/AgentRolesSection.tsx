// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {Autocomplete, CircularProgress, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import {useCallback, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetAgentRoles from '../../../api/useGetAgentRoles';
import useAgentRoutes from '../../../hooks/useAgentRoutes';

interface AgentRolesSectionProps {
  agentId: string;
}

export default function AgentRolesSection({agentId}: AgentRolesSectionProps): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetAgentRoles(agentId, {limit: 100, offset: 0});
  const roles = data?.roles ?? [];

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  return (
    <SettingsCard
      title={t('agents:edit.access.roles.title', 'Roles')}
      description={
        <Trans
          i18nKey="agents:edit.access.roles.description"
          defaults="Roles assigned to this agent, directly or through its groups. Manage assignments from the <manageLink>Roles page</manageLink>."
          components={{manageLink: <Link to={routes.roles.list()} />}}
        />
      }
    >
      {isLoading ? (
        <CircularProgress size={20} />
      ) : error ? (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="agents:edit.access.roles.error"
          fallbackDefaultValue="Failed to load roles for this agent."
          onRetry={() => void refetch()}
        />
      ) : (
        <FormControl fullWidth>
          <FormLabel htmlFor="agent-roles">{t('agents:edit.access.roles.label', 'Roles')}</FormLabel>
          <Autocomplete
            multiple
            readOnly
            disableClearable
            forcePopupIcon={false}
            fullWidth
            options={[]}
            value={roles}
            renderInput={(params) => (
              <TextField
                {...params}
                id="agent-roles"
                placeholder={
                  roles.length === 0
                    ? t('agents:edit.access.roles.empty', 'This agent does not have any roles assigned.')
                    : undefined
                }
              />
            )}
          />
        </FormControl>
      )}
    </SettingsCard>
  );
}
