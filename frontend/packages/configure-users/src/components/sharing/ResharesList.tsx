// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useGetOrganizationUnitsByIds} from '@thunderid/configure-organization-units';
import {useGetSharingPolicies, type SharingPolicy} from '@thunderid/configure-resource-sharing';
import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {CornerDownRight} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

interface ResharesListProps {
  /** The user's name, as the screen shows it. */
  userName: string;
  /** The user's path relative to the server URL, e.g. `users/<id>`. */
  resourcePath: string;
}

function unitIdsOf(policy: SharingPolicy): string[] {
  const children = policy.childUnits;
  if (children?.mode === 'selected') return children.units.map((unit) => unit.ouId);
  if (children?.mode === 'all') return children.excludedOuIds ?? [];
  return [];
}

/**
 * The shares other units made of the user, read-only. A unit the user is shared with "this unit only"
 * can pass the user on to units beneath it; each unit manages its own from its own context.
 */
export default function ResharesList({userName, resourcePath}: ResharesListProps): JSX.Element | null {
  const {t} = useTranslation('users');
  const {data: policies} = useGetSharingPolicies(resourcePath);
  const reshares = useMemo(() => (policies ?? []).filter((policy) => policy.stage === 'reshare'), [policies]);
  const ids = useMemo(
    () => [...new Set(reshares.flatMap((policy) => [policy.initiatingOuId ?? '', ...unitIdsOf(policy)]))],
    [reshares],
  );
  const unitsById = useGetOrganizationUnitsByIds(ids);
  const nameOf = (ouId: string): string => unitsById.get(ouId)?.name ?? ouId;

  if (reshares.length === 0) return null;

  return (
    <SettingsCard
      title={t('sharing.reshares.title', 'Passed on by other units')}
      description={t(
        'sharing.reshares.description',
        'Units {{name}} is shared with can pass the user on to units beneath them. Each unit manages its own; they are shown here read-only.',
        {name: userName},
      )}
    >
      <Stack spacing={2}>
        {reshares.map((policy) => {
          const issuer = nameOf(policy.initiatingOuId ?? '');
          const children = policy.childUnits;
          return (
            <Box key={policy.id} sx={{border: '1px solid', borderColor: 'divider', borderRadius: '10px', p: 2}}>
              <Typography variant="subtitle2" sx={{mb: 1}}>
                {t('sharing.reshares.issuer', '{{unit}} passes it on to', {unit: issuer})}
              </Typography>
              <Stack spacing={1}>
                {children?.mode === 'all' && (
                  <Stack direction="row" alignItems="center" sx={{gap: 1, flexWrap: 'wrap'}}>
                    <CornerDownRight size={14} />
                    <Typography variant="body2">
                      {t('sharing.reshares.allChildUnits', 'All child units of {{unit}}', {unit: issuer})}
                    </Typography>
                    {(children.excludedOuIds ?? []).length > 0 && (
                      <Typography variant="caption" color="text.secondary">
                        {t('sharing.reshares.except', 'except {{units}}', {
                          units: (children.excludedOuIds ?? []).map(nameOf).join(', '),
                        })}
                      </Typography>
                    )}
                  </Stack>
                )}
                {children?.mode === 'selected' &&
                  children.units.map((unit) => (
                    <Stack key={unit.ouId} direction="row" alignItems="center" sx={{gap: 1, flexWrap: 'wrap'}}>
                      <CornerDownRight size={14} />
                      <Typography variant="body2">{nameOf(unit.ouId)}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {unit.reach === 'subtree'
                          ? t('sharing.reshares.subtree', 'And below')
                          : t('sharing.reshares.unit', 'This unit only')}
                      </Typography>
                    </Stack>
                  ))}
              </Stack>
            </Box>
          );
        })}
      </Stack>
    </SettingsCard>
  );
}
