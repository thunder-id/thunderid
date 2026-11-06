// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ToggleButton, ToggleButtonGroup, Tooltip} from '@wso2/oxygen-ui';
import {Info} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChildUnitReach} from '../models/organization-access';

export interface ReachToggleGroupProps {
  /**
   * Name of the organization unit the reach is for, used in the group's accessible name.
   */
  name: string;
  value: ChildUnitReach;
  onChange: (reach: ChildUnitReach) => void;
  disabled?: boolean;
}

/**
 * Picks how far a granted organization unit's access reaches: the unit alone, or the unit and every
 * unit below it. Each choice explains itself in a tooltip on its info icon.
 */
export default function ReachToggleGroup({
  name,
  value,
  onChange,
  disabled = false,
}: ReachToggleGroupProps): JSX.Element {
  const {t} = useTranslation();

  const choices: {reach: ChildUnitReach; label: string; hint: string}[] = [
    {
      reach: 'unit',
      label: t('resourceSharing:reach.unit', 'Only this OU'),
      hint: t('resourceSharing:reach.unitHint', "Its child organization units don't get access."),
    },
    {
      reach: 'subtree',
      label: t('resourceSharing:reach.subtree', 'Include all OUs below'),
      hint: t('resourceSharing:reach.subtreeHint', 'This OU and every OU below it, now and later.'),
    },
  ];

  return (
    <ToggleButtonGroup
      exclusive
      size="small"
      fullWidth
      value={value}
      disabled={disabled}
      // Read-only keeps the chosen reach marked, so the saved reach still reads at a glance.
      sx={{'& .MuiToggleButton-root.Mui-disabled.Mui-selected': {color: 'primary.main'}}}
      aria-label={t('resourceSharing:reach.label', 'Reach for {{name}}', {name})}
      onChange={(_event, reach: ChildUnitReach | null) => {
        if (reach) onChange(reach);
      }}
    >
      {choices.map((choice) => (
        // A tooltip can't open from a disabled button, so read-only drops it along with its icon.
        <Tooltip key={choice.reach} title={disabled ? '' : choice.hint} describeChild>
          <ToggleButton
            value={choice.reach}
            // One line from small screens up, where the toggle's column fits both labels; a phone may wrap.
            sx={{textTransform: 'none', textAlign: 'center', gap: 0.5, whiteSpace: {xs: 'normal', sm: 'nowrap'}}}
          >
            {choice.label}
            {!disabled && <Info size={14} aria-hidden />}
          </ToggleButton>
        </Tooltip>
      ))}
    </ToggleButtonGroup>
  );
}
