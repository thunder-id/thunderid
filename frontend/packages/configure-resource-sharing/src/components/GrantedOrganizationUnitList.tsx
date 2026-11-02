// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {Box, IconButton, Stack, ToggleButton, ToggleButtonGroup, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Trash2} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChildUnitReach} from '../models/organization-access';

export interface GrantedOrganizationUnit {
  ouId: string;
  /**
   * How far below the unit access reaches. Omit for units whose reach is fixed, such as the top
   * unit of another tree.
   */
  reach?: ChildUnitReach;
}

export interface GrantedOrganizationUnitListProps {
  units: GrantedOrganizationUnit[];
  unitsById: Map<string, OrganizationUnit>;
  readOnly: boolean;
  emptyLabel: string;
  /**
   * Shown under the unit's handle when the unit has no reach to choose.
   */
  fixedReachLabel?: string;
  onReachChange?: (ouId: string, reach: ChildUnitReach) => void;
  onRemove: (ouId: string) => void;
}

/**
 * The organization units a resource is granted to one by one, each with its reach and a revoke
 * action.
 */
export default function GrantedOrganizationUnitList({
  units,
  unitsById,
  readOnly,
  emptyLabel,
  fixedReachLabel = undefined,
  onReachChange = undefined,
  onRemove,
}: GrantedOrganizationUnitListProps): JSX.Element {
  const {t} = useTranslation();

  if (units.length === 0) {
    return (
      <Box sx={{border: '1px dashed', borderColor: 'divider', borderRadius: '10px', p: 3, textAlign: 'center'}}>
        <Typography variant="body2" color="text.secondary">
          {emptyLabel}
        </Typography>
      </Box>
    );
  }

  return (
    <Box
      component="ul"
      sx={{listStyle: 'none', m: 0, p: 0, border: '1px solid', borderColor: 'divider', borderRadius: '10px'}}
    >
      {units.map((unit, index) => {
        const ou = unitsById.get(unit.ouId);
        const name = ou?.name ?? unit.ouId;

        return (
          <Box
            component="li"
            key={unit.ouId}
            sx={{
              // A fixed grid keeps every row's reach toggle and revoke action in the same column, whatever
              // the length of the unit's name or of the hint under its reach.
              display: 'grid',
              gridTemplateColumns: {xs: 'minmax(0, 1fr) 40px', sm: 'minmax(0, 1fr) 300px 40px'},
              gridTemplateAreas: {xs: '"name action" "reach reach"', sm: '"name reach action"'},
              alignItems: 'center',
              columnGap: 2,
              rowGap: 1,
              px: 2,
              py: 1.5,
              borderTop: index === 0 ? 'none' : '1px solid',
              borderColor: 'divider',
            }}
          >
            <Box sx={{gridArea: 'name', minWidth: 0}}>
              <Typography variant="body1" fontWeight={600} noWrap>
                {name}
              </Typography>
              <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
                {ou?.handle ?? unit.ouId}
                {!unit.reach && fixedReachLabel ? ` · ${fixedReachLabel}` : ''}
              </Typography>
            </Box>
            {unit.reach && (
              <Stack spacing={0.5} sx={{gridArea: 'reach', alignItems: 'flex-start'}}>
                <ToggleButtonGroup
                  exclusive
                  size="small"
                  value={unit.reach}
                  disabled={readOnly}
                  // Read-only keeps the chosen reach marked, so the saved reach still reads at a glance.
                  sx={{'& .MuiToggleButton-root.Mui-disabled.Mui-selected': {color: 'primary.main'}}}
                  aria-label={t('resourceSharing:reach.label', 'Reach for {{name}}', {name})}
                  onChange={(_event, reach: ChildUnitReach | null) => {
                    if (reach) onReachChange?.(unit.ouId, reach);
                  }}
                >
                  <ToggleButton value="unit" sx={{textTransform: 'none'}}>
                    {t('resourceSharing:reach.unit', 'This unit only')}
                  </ToggleButton>
                  <ToggleButton value="subtree" sx={{textTransform: 'none'}}>
                    {t('resourceSharing:reach.subtree', 'And below')}
                  </ToggleButton>
                </ToggleButtonGroup>
                <Typography variant="caption" color="text.secondary">
                  {unit.reach === 'unit'
                    ? t('resourceSharing:reach.unitHint', "Units below it don't get access.")
                    : t('resourceSharing:reach.subtreeHint', 'Includes every unit below it, now and later.')}
                </Typography>
              </Stack>
            )}
            {!readOnly && (
              <Tooltip title={t('resourceSharing:revoke', 'Revoke access for {{name}}', {name})}>
                <IconButton
                  color="error"
                  sx={{gridArea: 'action', justifySelf: 'end'}}
                  aria-label={t('resourceSharing:revoke', 'Revoke access for {{name}}', {name})}
                  onClick={() => onRemove(unit.ouId)}
                >
                  <Trash2 size={16} />
                </IconButton>
              </Tooltip>
            )}
          </Box>
        );
      })}
    </Box>
  );
}
