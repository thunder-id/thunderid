// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  useApplyVersion,
  useAutoApply,
  usePipeline,
  useProjectsPending,
  useSetAutoApply,
  type Project,
} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {useCapabilitiesContext, useToast} from '@thunderid/contexts';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Switch,
  Typography,
  alpha,
} from '@wso2/oxygen-ui';
import {CircleAlert} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Turns the organization's auto deploy on or off: every saved change then deploys to the first gateway on its
 * own. Turned on, what the first gateway lacks is deployed first, and it is turned on only once that is all
 * there; anything that fails is told in an error toast, and the setting stays as it was.
 */
export default function AutoDeploySwitch(): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const pipeline = usePipeline();
  const {projects} = useProjectSelection();
  const autoApply = useAutoApply();
  const setAutoApply = useSetAutoApply();
  const applyVersion = useApplyVersion();
  const canUpdate = useCapabilitiesContext().can('serverconfig', 'update');
  // The setting asked for, waiting for the person to confirm it.
  const [confirming, setConfirming] = useState<boolean | null>(null);
  const [saving, setSaving] = useState<boolean | null>(null);
  const auto = saving ?? autoApply.data?.enabled ?? false;
  const busy = setAutoApply.isPending || applyVersion.isPending;
  const first = pipeline.data?.stages[0];
  const firstTag = first?.name ?? '';
  const projectIds = useMemo(() => projects.map((project: Project) => project.id), [projects]);
  const pending = useProjectsPending(first?.gateway.id ?? '', projectIds);
  const pendingCount = projects.reduce(
    (sum: number, project: Project) => sum + (pending.byProject[project.id] ?? 0),
    pending.shared,
  );
  const hasPending = pendingCount > 0;

  const ask = (next: boolean | null): void => setConfirming(next);
  // A change that did not go through is told in a toast; the dialog stays open, with the setting as it was.
  const fail = (message: string): void => {
    setSaving(null);
    showToast(message, 'error');
  };

  const confirm = async (): Promise<void> => {
    const next = confirming;
    if (next === null) return;
    if (next && first && hasPending) {
      try {
        const result = await applyVersion.mutateAsync({gatewayId: first.gateway.id, data: {version: 'latest'}});
        const refused = result.import?.summary?.failed ?? 0;
        if (refused > 0) {
          fail(
            t('home:organization.autoSyncDeployPartial', {
              count: refused,
              target: firstTag,
              defaultValue_one: '{{target}} refused {{count}} change, so auto deploy is still off.',
              defaultValue_other: '{{target}} refused {{count}} changes, so auto deploy is still off.',
            }),
          );
          return;
        }
      } catch {
        fail(
          t(
            'home:organization.autoSyncDeployFailed',
            'The pending changes could not be deployed to {{target}}, so auto deploy is still off.',
            {target: firstTag},
          ),
        );
        return;
      }
    }
    setSaving(next);
    try {
      await setAutoApply.mutateAsync(next);
    } catch {
      fail(
        next && hasPending
          ? t(
              'home:organization.autoSyncFailedAfterDeploy',
              'The pending changes are deployed to {{target}}, but auto deploy could not be turned on.',
              {target: firstTag},
            )
          : t('home:organization.autoSyncFailed', 'Auto deploy could not be changed.'),
      );
      return;
    }
    if (next && hasPending) {
      showToast(
        t(
          'home:organization.autoSyncDeployed',
          'The pending changes are deployed to {{target}}, and auto deploy is on.',
          {
            target: firstTag,
          },
        ),
        'success',
      );
    }
    setSaving(null);
    setConfirming(null);
  };

  return (
    <>
      {/* Its own outlined pill, warmed while on, so the pipeline-wide setting stands out in the header. */}
      <Box sx={{pr: 2}}>
        <Stack
          component="label"
          direction="row"
          alignItems="center"
          spacing={0.25}
          sx={{
            pl: 0.5,
            pr: 1.5,
            border: 1,
            borderRadius: '20px',
            cursor: 'pointer',
            borderColor: auto ? 'primary.main' : 'divider',
            bgcolor: (theme) => (auto ? alpha(theme.palette.primary.main, 0.1) : 'transparent'),
            transition: 'background-color 0.2s, border-color 0.2s',
          }}
        >
          <Switch
            checked={auto}
            disabled={
              !canUpdate ||
              !first ||
              autoApply.isLoading ||
              autoApply.isError ||
              (autoApply.data?.locked ?? false) ||
              busy
            }
            onChange={(_event, checked: boolean) => ask(checked)}
            slotProps={{
              input: {
                'aria-label': t('home:organization.autoSyncSwitch', 'Auto deploy configuration to {{target}}', {
                  target: firstTag,
                }),
              },
            }}
            data-testid="auto-deploy-switch"
          />
          <Typography variant="body2" sx={{fontWeight: 600, whiteSpace: 'nowrap'}}>
            {t('home:pipeline.autoDeploy', 'Auto deploy')}
          </Typography>
        </Stack>
      </Box>

      <Dialog
        open={confirming !== null}
        onClose={() => {
          if (!busy) ask(null);
        }}
        fullWidth
        maxWidth="sm"
        data-testid="auto-deploy-confirm"
      >
        {confirming !== null && (
          <>
            <DialogTitle>
              {confirming
                ? t('home:organization.autoSyncOnTitle', 'Turn on auto deploy to {{target}}?', {target: firstTag})
                : t('home:organization.autoSyncOffTitle', 'Turn off auto deploy to {{target}}?', {target: firstTag})}
            </DialogTitle>
            <DialogContent>
              <Typography variant="body2" color="text.secondary">
                {confirming
                  ? t(
                      'home:organization.autoSyncOnBody',
                      'Every change saved in this console will deploy to {{target}}.',
                      {
                        target: firstTag,
                      },
                    )
                  : t(
                      'home:organization.autoSyncOffBody',
                      'Saved changes will no longer deploy to {{target}} on their own. They wait here until you deploy them.',
                      {target: firstTag},
                    )}
              </Typography>
              {/* One setting for the whole organization, so the person knows it reaches past this project. */}
              <Alert severity="info" sx={{mt: 1.5}} data-testid="auto-deploy-all-projects">
                {t('home:pipeline.autoDeployAllProjects', 'This applies to all projects in the organization.')}
              </Alert>
              {confirming && hasPending && (
                <Stack
                  direction="row"
                  spacing={0.75}
                  alignItems="center"
                  sx={{mt: 1.5, color: 'warning.main'}}
                  data-testid="auto-deploy-confirm-pending"
                >
                  <CircleAlert size={14} />
                  <Typography variant="body2" color="inherit">
                    {t('home:organization.autoSyncBehind', {
                      count: pendingCount,
                      defaultValue_one: '{{count}} change behind',
                      defaultValue_other: '{{count}} changes behind',
                    })}
                  </Typography>
                </Stack>
              )}
              {confirming && pending.isError && (
                <Alert severity="warning" sx={{mt: 1.5}} data-testid="auto-deploy-confirm-unread">
                  {t(
                    'home:organization.autoSyncPendingUnread',
                    'What {{target}} is behind on could not be read, so nothing is deployed before auto deploy turns on.',
                    {target: firstTag},
                  )}
                </Alert>
              )}
            </DialogContent>
            <DialogActions>
              <Button onClick={() => ask(null)} disabled={busy}>
                {t('home:organization.pickCancel', 'Cancel')}
              </Button>
              <Button
                variant="contained"
                onClick={() => void confirm()}
                disabled={busy || (confirming && pending.isLoading)}
                startIcon={busy ? <CircularProgress size={14} color="inherit" /> : undefined}
                data-testid="auto-deploy-confirm-button"
              >
                {confirming
                  ? hasPending
                    ? t('home:organization.autoSyncOnDeploy', 'Deploy and turn on')
                    : t('home:organization.autoSyncOn', 'Turn on')
                  : t('home:organization.autoSyncOff', 'Turn off')}
              </Button>
            </DialogActions>
          </>
        )}
      </Dialog>
    </>
  );
}
