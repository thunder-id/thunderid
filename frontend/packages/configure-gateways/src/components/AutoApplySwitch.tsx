// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {Box, Stack, Switch, Tooltip, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useAutoApply, useSetAutoApply} from '../api/usePipeline';

export interface AutoApplySwitchProps {
  /** The environment a saved change deploys to: the first of the pipeline. */
  target: string;
}

/**
 * Turns deploying each saved change to the first environment on or off. Later environments are reached
 * by promoting only, so the switch belongs to the first.
 */
export default function AutoApplySwitch({target}: AutoApplySwitchProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const {showToast} = useToast();
  const setting = useAutoApply();
  const save = useSetAutoApply();
  const enabled = setting.data?.enabled ?? false;
  const locked = setting.data?.locked ?? false;
  const label = t('autoApply.label', 'Auto-sync configuration to {{target}}', {target});

  const toggle = (next: boolean): void => {
    save.mutate(next, {
      onError: () => showToast(t('autoApply.failed', 'Auto-sync could not be changed.'), 'error'),
    });
  };

  let hint = t('autoApply.off', 'Saved changes wait here until you deploy them.');
  if (setting.isError) hint = t('autoApply.unreadable', 'Whether auto-sync is on could not be read.');
  else if (enabled) hint = t('autoApply.on', 'Saved changes deploy to {{target}} within seconds.', {target});

  const control = (
    <Box component="span">
      <Switch
        checked={enabled}
        onChange={(_event, checked: boolean) => toggle(checked)}
        disabled={locked || setting.isLoading || setting.isError || save.isPending}
        slotProps={{input: {'aria-label': label}}}
        data-testid="auto-apply-switch"
      />
    </Box>
  );
  return (
    <Stack direction="row" spacing={1} alignItems="flex-start" data-testid="auto-apply">
      {/* The tooltip stays mounted either way, so the switch is not replaced once the setting is read. */}
      <Tooltip
        title={
          locked ? t('autoApply.locked', 'Set in the deployment configuration, so it cannot be changed here.') : ''
        }
      >
        {control}
      </Tooltip>
      <Box sx={{pt: 0.75}}>
        <Typography variant="body2" fontWeight={600}>
          {label}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {hint}
        </Typography>
      </Box>
    </Stack>
  );
}
