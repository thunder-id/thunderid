// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {Box, Button, IconButton, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ReachToggleGroup from './ReachToggleGroup';
import type {ChildUnitReach} from '../models/organization-access';

export interface GrantedOrganizationUnit {
  ouId: string;
  /**
   * How far below the unit access reaches. Omit for units whose reach is fixed, such as the top
   * unit of another tree.
   */
  reach?: ChildUnitReach;
}

/**
 * The terms each granted unit holds the resource on, with a button on each row that opens them under it.
 */
export interface GrantedUnitTerms {
  /**
   * What the button is called to assistive technology, which names the unit whose terms it opens.
   */
  editLabel: (name: string) => string;
  /**
   * The unit whose terms are open, if any.
   */
  expandedOuId?: string | null;
  /**
   * Opens or closes the unit's terms, which appear under its row.
   */
  onEdit: (ouId: string, name: string) => void;
  /**
   * Draws the unit's terms under its row while they are open.
   */
  renderPanel?: (ouId: string, name: string) => ReactNode;
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
  /**
   * Content shown under a unit's row, across its full width.
   */
  renderUnitExtra?: (unit: GrantedOrganizationUnit) => ReactNode;
  onRemove: (ouId: string) => void;
  /**
   * Shows a way to open each unit's terms, when the resource's units hold terms of their own.
   */
  terms?: GrantedUnitTerms;
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
  renderUnitExtra = undefined,
  onRemove,
  terms = undefined,
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
        const extra = renderUnitExtra?.(unit);
        const panel =
          terms?.renderPanel && terms.expandedOuId === unit.ouId ? terms.renderPanel(unit.ouId, name) : null;

        return (
          <Box
            component="li"
            key={unit.ouId}
            sx={{
              // A fixed grid keeps every row's reach toggle and revoke action in the same column, whatever
              // the length of the unit's name.
              display: 'grid',
              gridTemplateColumns: terms
                ? {xs: 'minmax(0, 1fr) 40px', sm: 'minmax(0, 1fr) 360px auto 40px'}
                : {xs: 'minmax(0, 1fr) 40px', sm: 'minmax(0, 1fr) 360px 40px'},
              gridTemplateAreas: terms
                ? {xs: '"name action" "reach reach" "terms terms"', sm: '"name reach terms action"'}
                : {xs: '"name action" "reach reach"', sm: '"name reach action"'},
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
              <Box sx={{gridArea: 'reach'}}>
                <ReachToggleGroup
                  name={name}
                  value={unit.reach}
                  disabled={readOnly}
                  onChange={(reach) => onReachChange?.(unit.ouId, reach)}
                />
              </Box>
            )}
            {terms && (
              <Button
                size="small"
                variant="text"
                aria-label={terms.editLabel(name)}
                aria-expanded={terms.expandedOuId === unit.ouId}
                startIcon={<Pencil size={14} />}
                onClick={() => terms.onEdit(unit.ouId, name)}
                sx={{gridArea: 'terms', justifySelf: 'start', textTransform: 'none'}}
              >
                {t('resourceSharing:terms.edit', 'Edit terms')}
              </Button>
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
            {(Boolean(extra) || Boolean(panel)) && (
              <Box sx={{gridColumn: '1 / -1'}}>
                {extra}
                {panel}
              </Box>
            )}
          </Box>
        );
      })}
    </Box>
  );
}
