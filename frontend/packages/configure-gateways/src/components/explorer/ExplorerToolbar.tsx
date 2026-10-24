// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, MenuItem, Select, Stack, ToggleButton, ToggleButtonGroup, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Gateway} from '../../models/gateway';

/** Up to this many gateways switch with a segmented control; more need a select. */
const SEGMENTED_LIMIT = 4;

export interface ExplorerToolbarProps {
  gateway: Gateway;
  gateways: Gateway[];
  /** The label of the version the gateway runs. */
  version?: string;
  /** "27 variables, 9 secrets". */
  counts?: string;
  onSwitch: (gatewayId: string) => void;
}

/** The explorer's top bar: which gateway this is, what it runs, and the switch to another. */
export default function ExplorerToolbar({
  gateway,
  gateways,
  version = undefined,
  counts = undefined,
  onSwitch,
}: ExplorerToolbarProps): JSX.Element {
  const {t} = useTranslation('deploy');
  return (
    <Stack direction="row" alignItems="center" spacing={1.5} sx={{px: 4, pb: 2, flexShrink: 0}}>
      <Typography variant="subtitle1" fontWeight={600} noWrap>
        {gateway.name}
      </Typography>
      {gateway.baseUrl && (
        <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}} noWrap>
          {gateway.baseUrl}
        </Typography>
      )}
      {version && <Chip size="small" label={version} sx={{fontFamily: 'monospace', height: 20, fontSize: 11}} />}
      <Box sx={{flex: 1}} />
      {counts && (
        <Typography variant="caption" color="text.secondary" noWrap>
          {counts}
        </Typography>
      )}
      {gateways.length > 1 && gateways.length <= SEGMENTED_LIMIT && (
        <ToggleButtonGroup
          size="small"
          exclusive
          value={gateway.id}
          onChange={(_event, value: string | null) => {
            if (value) onSwitch(value);
          }}
          aria-label={t('explorer.switch', 'Gateway')}
          sx={{'& .MuiToggleButton-root': {borderRadius: 100, px: 1.5, py: 0.25, textTransform: 'none'}}}
        >
          {gateways.map((entry: Gateway) => (
            <ToggleButton key={entry.id} value={entry.id}>
              {entry.name}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      )}
      {gateways.length > SEGMENTED_LIMIT && (
        <Select
          size="small"
          value={gateway.id}
          onChange={(event) => onSwitch(String(event.target.value))}
          inputProps={{'aria-label': t('explorer.switch', 'Gateway')}}
          sx={{minWidth: 140}}
        >
          {gateways.map((entry: Gateway) => (
            <MenuItem key={entry.id} value={entry.id}>
              {entry.name}
            </MenuItem>
          ))}
        </Select>
      )}
    </Stack>
  );
}
