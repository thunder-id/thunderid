// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChangeType, GatewayDiff, ResourceChange} from '../models/gateway';

const CHANGE_COLORS: Record<ChangeType, 'success' | 'warning' | 'error' | 'default'> = {
  added: 'success',
  updated: 'warning',
  deleted: 'error',
  unchanged: 'default',
};

export interface ResourceDiffListProps {
  diff: GatewayDiff;
}

/**
 * Lists what a diff changes, grouped by resource type. Unchanged resources are left out, and a
 * deletion is marked so it stands out, since it removes something from a running gateway.
 */
export default function ResourceDiffList({diff}: ResourceDiffListProps): JSX.Element {
  const {t} = useTranslation();

  const groups = useMemo((): [string, ResourceChange[]][] => {
    const byType = new Map<string, ResourceChange[]>();
    (diff.changes ?? [])
      .filter((change: ResourceChange) => change.change !== 'unchanged')
      .forEach((change: ResourceChange) => {
        byType.set(change.resourceType, [...(byType.get(change.resourceType) ?? []), change]);
      });
    return [...byType.entries()].sort(([a], [b]) => a.localeCompare(b));
  }, [diff]);

  const changeLabel = (change: ChangeType): string => {
    switch (change) {
      case 'added':
        return t('gateways:diff.added', 'Added');
      case 'updated':
        return t('gateways:diff.updated', 'Updated');
      case 'deleted':
        return t('gateways:diff.deleted', 'Deleted');
      default:
        return t('gateways:diff.unchanged', 'Unchanged');
    }
  };

  if (groups.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{py: 2}}>
        {t('gateways:diff.noChanges', 'Nothing would change. The gateway already holds this configuration.')}
      </Typography>
    );
  }

  return (
    <Stack spacing={2}>
      {groups.map(([resourceType, changes]) => (
        <Box key={resourceType}>
          <Typography variant="subtitle2" sx={{mb: 1}}>
            {resourceType}
          </Typography>
          <Stack spacing={0.5}>
            {changes.map((change: ResourceChange) => (
              <Stack
                key={`${change.resourceType}-${change.id}`}
                direction="row"
                spacing={1.5}
                alignItems="center"
                data-change={change.change}
                sx={{
                  px: 1.5,
                  py: 0.75,
                  borderLeft: 3,
                  borderLeftColor: change.change === 'deleted' ? 'error.main' : `${CHANGE_COLORS[change.change]}.main`,
                  borderRadius: 1,
                  bgcolor: change.change === 'deleted' ? 'error.lighter' : 'action.hover',
                }}
              >
                <Chip size="small" color={CHANGE_COLORS[change.change]} label={changeLabel(change.change)} />
                <Box sx={{minWidth: 0}}>
                  <Typography
                    variant="body2"
                    noWrap
                    sx={{
                      fontWeight: 500,
                      textDecoration: change.change === 'deleted' ? 'line-through' : 'none',
                    }}
                  >
                    {change.name ?? change.id}
                  </Typography>
                  {change.name && change.name !== change.id && (
                    <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
                      {change.id}
                    </Typography>
                  )}
                </Box>
              </Stack>
            ))}
          </Stack>
        </Box>
      ))}
    </Stack>
  );
}
