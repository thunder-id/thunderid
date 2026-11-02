// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {Autocomplete, CircularProgress, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import {useCallback, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetAgentGroups from '../../../api/useGetAgentGroups';
import useAgentRoutes from '../../../hooks/useAgentRoutes';

interface AgentGroupsSectionProps {
  agentId: string;
}

export default function AgentGroupsSection({agentId}: AgentGroupsSectionProps): JSX.Element {
  const routes = useAgentRoutes();
  const {t} = useTranslation();
  const {data, isLoading, error, refetch} = useGetAgentGroups(agentId, {limit: 100, offset: 0});
  const groups = data?.groups ?? [];

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  return (
    <SettingsCard
      title={t('agents:edit.access.groups.title', 'Groups')}
      description={
        <Trans
          i18nKey="agents:edit.access.groups.description"
          defaults="Groups this agent belongs to. Manage membership from the <manageLink>Groups page</manageLink>."
          components={{manageLink: <Link to={routes.groups.list()} />}}
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
          fallbackKey="agents:edit.access.groups.error"
          fallbackDefaultValue="Failed to load groups for this agent."
          onRetry={() => void refetch()}
        />
      ) : (
        <FormControl fullWidth>
          <FormLabel htmlFor="agent-groups">{t('agents:edit.access.groups.label', 'Groups')}</FormLabel>
          <Autocomplete
            multiple
            readOnly
            disableClearable
            forcePopupIcon={false}
            fullWidth
            options={[]}
            value={groups.map((group) => group.name)}
            renderInput={(params) => (
              <TextField
                {...params}
                id="agent-groups"
                placeholder={
                  groups.length === 0
                    ? t('agents:edit.access.groups.empty', 'This agent does not belong to any groups.')
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
