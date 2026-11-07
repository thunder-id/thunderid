// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Button, Stack, Switch, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Info} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useAutoApply, useSetAutoApply, useSetProjectAutoSync} from '../api/usePipeline';
import useProjectStatus from '../api/useProjectStatus';
import useProjectScope from '../contexts/ProjectScope';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import {errorCodeOf, SETTINGS_DECLARATIVE} from '../utils/apiError';

export interface AutoApplySwitchProps {
  /** The gateway a saved change deploys to: the first of the pipeline. */
  target: string;
  /** The first gateway's target id, which a project's own auto-sync is read for. */
  targetId?: string;
  /** Shows the label alone, without the line saying what the switch does, such as on a narrow card. */
  compact?: boolean;
}

/** What the toast says when auto-sync could not be changed: the catalog's message for the error's code, if any. */
function failedMessage(tForErrors: ReturnType<typeof useGatewayErrorTranslator>, error: unknown): string {
  return getErrorMessage(error as Error, tForErrors, 'deploy:autoApply.failed', 'Auto-sync could not be changed.');
}

/**
 * Turns a project's own auto-sync on or off, or has it follow the organization's default again. Shared
 * resources go with any project's auto-sync, which the hint says.
 */
function ProjectAutoSyncSwitch({
  project,
  target,
  targetId,
}: {
  project: string;
  target: string;
  targetId: string;
}): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const {showToast} = useToast();
  const status = useProjectStatus(targetId);
  const save = useSetProjectAutoSync();
  const autoSync = status.data?.autoSync;
  // While a change is saved the switch shows it; a refused one puts the switch back as it was.
  const [saving, setSaving] = useState<boolean | null>(null);
  const enabled = saving ?? autoSync?.enabled ?? false;
  const inherited = autoSync?.inherited ?? true;
  const label = t('autoApply.projectLabel', 'Auto-sync from configuration');
  // Set once a write is refused because the deployment configuration sets auto-sync.
  const [locked, setLocked] = useState(false);

  const set = (next: boolean | null): void => {
    setSaving(next);
    save
      .mutateAsync({project, enabled: next})
      .catch((error: unknown) => {
        if (errorCodeOf(error) === SETTINGS_DECLARATIVE) {
          setLocked(true);
          return;
        }
        showToast(failedMessage(tForErrors, error), 'error');
      })
      .finally(() => setSaving(null));
  };

  return (
    <Stack direction="row" spacing={1} alignItems="flex-start" data-testid="auto-apply">
      <Tooltip
        title={
          locked ? t('autoApply.locked', 'Set in the deployment configuration, so it cannot be changed here.') : ''
        }
      >
        <Box component="span">
          <Switch
            checked={enabled}
            onChange={(_event, checked: boolean) => set(checked)}
            disabled={!autoSync || locked || save.isPending}
            slotProps={{input: {'aria-label': label}}}
            data-testid="auto-apply-switch"
          />
        </Box>
      </Tooltip>
      <Box sx={{pt: 0.75, minWidth: 0}}>
        <Stack direction="row" spacing={0.5} alignItems="center">
          <Typography variant="body2" fontWeight={600}>
            {label}
          </Typography>
          <Tooltip
            title={t(
              'autoApply.sharedInfo',
              "Shared resources, such as translations, branding, common flows and users, go with any project's auto-sync, so they can reach projects that have it off.",
            )}
          >
            <Box
              component="span"
              tabIndex={0}
              role="img"
              aria-label={t('autoApply.sharedInfoLabel', 'About shared resources')}
              sx={{display: 'flex', color: 'text.secondary', cursor: 'help'}}
              data-testid="auto-apply-info"
            >
              <Info size={14} />
            </Box>
          </Tooltip>
        </Stack>
        <Typography variant="caption" color="text.secondary" component="div">
          {t('autoApply.projectHint', 'Changes to this project and shared resources deploy to {{target}} on save.', {
            target,
          })}
        </Typography>
        {autoSync &&
          (inherited ? (
            <Typography variant="caption" color="text.secondary" component="div" data-testid="auto-apply-inherited">
              {t('autoApply.inherited', 'Follows the organization default')}
            </Typography>
          ) : (
            <Button
              variant="text"
              size="small"
              onClick={() => set(null)}
              disabled={locked || save.isPending}
              sx={{px: 0, minHeight: 0}}
              data-testid="auto-apply-use-default"
            >
              {t('autoApply.useDefault', 'Use organization default')}
            </Button>
          ))}
      </Box>
    </Stack>
  );
}

/**
 * Turns deploying each saved change to the first gateway on or off. Later gateways are reached
 * by promoting only, so the switch belongs to the first. Across the organization it is the default for
 * projects; within a project it is the project's own.
 */
export default function AutoApplySwitch({
  target,
  targetId = undefined,
  compact = false,
}: AutoApplySwitchProps): JSX.Element {
  const project = useProjectScope();
  if (project && targetId) return <ProjectAutoSyncSwitch project={project} target={target} targetId={targetId} />;
  return <OrganizationAutoApplySwitch target={target} compact={compact} />;
}

/** The organization's auto-sync: the default for projects that set none of their own. */
function OrganizationAutoApplySwitch({target, compact}: {target: string; compact: boolean}): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const {showToast} = useToast();
  const setting = useAutoApply();
  const save = useSetAutoApply();
  const locked = setting.data?.locked ?? false;
  const label = t('autoApply.label', 'Default for projects: auto-sync configuration to {{target}}', {target});

  const [saving, setSaving] = useState<boolean | null>(null);
  const toggle = (next: boolean): void => {
    setSaving(next);
    save
      .mutateAsync(next)
      .catch((error: unknown) => showToast(failedMessage(tForErrors, error), 'error'))
      .finally(() => setSaving(null));
  };

  const enabled = saving ?? setting.data?.enabled ?? false;
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
        {(!compact || setting.isError) && (
          <Typography variant="caption" color="text.secondary">
            {hint}
          </Typography>
        )}
      </Box>
    </Stack>
  );
}
