// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DiffSummaryChips from './DiffSummaryChips';
import MissingVariablesNotice from './MissingVariablesNotice';
import ResourceDiffList from './ResourceDiffList';
import useCheckVariables from '../api/useCheckVariables';
import useGetEnvironments from '../api/useGetEnvironments';
import useGetPromotionPreview from '../api/useGetPromotionPreview';
import usePromote from '../api/usePromote';
import type {Diff, Environment, ResourceChange} from '../models/promotion';

interface PromoteDialogProps {
  open: boolean;
  fromEnvId: string;
  fromEnvName: string;
  toEnvId: string;
  toEnvName: string;
  /**
   * Whether the target environment's Data Plane is connected. A promotion writes to the target's
   * Control Plane and does not need it, but applying to the Data Plane afterwards does.
   */
  toDataPlaneConnected?: boolean;
  /** Labels the action as a demotion when promoting to a lower ranked environment. */
  isDemotion?: boolean;
  /** Version of the source environment to promote. Defaults to its latest. */
  version?: string;
  onClose: () => void;
}

/**
 * Reviews a promotion before it happens: shows the diff against the target environment, lets the
 * user reject individual changes, and promotes the remaining selection.
 */
export default function PromoteDialog({
  open,
  fromEnvId,
  fromEnvName,
  toEnvId,
  toEnvName,
  toDataPlaneConnected = false,
  isDemotion = false,
  version = undefined,
  onClose,
}: PromoteDialogProps): JSX.Element {
  const {t} = useTranslation();
  const {data: diff, isLoading, error} = useGetPromotionPreview(open ? fromEnvId : '', open ? toEnvId : '', version);
  const promote = usePromote();
  const {data: environments} = useGetEnvironments();
  const {data: variableStatus} = useCheckVariables(open ? fromEnvId : '', version);

  // What the user changed in this dialog, keyed by resource, holding whether they want it promoted.
  // Only their explicit toggles are tracked: everything else falls back to what the environment
  // remembers, so the set stays correct as the preview loads without resynchronizing from the diff,
  // and without an effect that would fight the user's clicks.
  const [toggled, setToggled] = useState<Map<string, boolean>>(new Map());
  // Off by default. A promotion writes the target's Control Plane; applying it to the Data Plane is a
  // separate decision about changing something that is serving traffic, and it should be taken
  // deliberately rather than carried along by the promotion.
  //
  // Applying reaches the target's Data Plane over the connection it holds open to this Control Plane.
  // With no connection there is nothing to apply to, so the promotion is offered on its own: the
  // configuration still lands in the target's Control Plane and is applied once it reconnects.
  const [applyNow, setApplyNow] = useState<boolean>(false);
  const canApply: boolean = toDataPlaneConnected;

  const changedKeys: string[] = useMemo(
    () =>
      (diff?.changes ?? [])
        .filter((change: ResourceChange) => change.change !== 'unchanged')
        .map((change: ResourceChange) => change.key),
    [diff],
  );

  // A resource held back on an earlier run starts deselected, so the decision does not have to be
  // made again every time.
  const rememberedExclusions: Set<string> = useMemo(
    () => new Set(environments?.environments?.find((env: Environment) => env.id === toEnvId)?.excluded ?? []),
    [environments, toEnvId],
  );

  const selectedKeys: Set<string> = useMemo(
    () =>
      new Set(
        changedKeys.filter((key: string) =>
          toggled.has(key) ? (toggled.get(key) ?? false) : !rememberedExclusions.has(key),
        ),
      ),
    [changedKeys, toggled, rememberedExclusions],
  );

  const handleToggle = (key: string): void => {
    setToggled((current: Map<string, boolean>) => {
      const next = new Map(current);
      next.set(key, !selectedKeys.has(key));
      return next;
    });
  };

  const handlePromote = (): void => {
    // The selection is always sent, never left to a default. It is what the environment remembers for
    // next time, so omitting it when everything is selected would leave an earlier exclusion in place
    // and quietly ignore the user reselecting the resource.
    promote.mutate(
      {
        fromEnv: fromEnvId,
        toEnv: toEnvId,
        version,
        selection: Array.from(selectedKeys),
        apply: applyNow && canApply,
      },
      {onSuccess: () => onClose()},
    );
  };

  const title: string = isDemotion
    ? t('promotions:demote.title', 'Demote configuration')
    : t('promotions:promote.title', 'Promote configuration');

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
          {t('promotions:promote.description', 'Review what will change in {{target}}, taken from {{source}}.', {
            source: fromEnvName,
            target: toEnvName,
          })}
        </Typography>

        {isLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
            <CircularProgress />
          </Box>
        )}

        {error && (
          <Alert severity="error" sx={{mb: 2}}>
            {error.message || t('promotions:preview.error', 'Failed to load the promotion preview')}
          </Alert>
        )}

        <MissingVariablesNotice
          missing={variableStatus?.missing ?? []}
          missingSecrets={variableStatus?.missingSecrets ?? []}
        />

        {diff && <PromotionBody diff={diff} selectedKeys={selectedKeys} onToggle={handleToggle} />}
      </DialogContent>
      <DialogActions>
        <FormControlLabel
          sx={{mr: 'auto'}}
          disabled={!canApply}
          control={
            <Checkbox
              checked={applyNow && canApply}
              disabled={!canApply}
              onChange={(event) => {
                setApplyNow(event.target.checked);
              }}
            />
          }
          label={
            canApply
              ? t('promotions:promote.applyNow', 'Apply to the target data plane now')
              : t('promotions:promote.dataPlaneOffline', '{{target}} is offline, so it cannot be applied to yet', {
                  target: toEnvName,
                })
          }
        />
        <Button onClick={onClose} disabled={promote.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={handlePromote}
          disabled={promote.isPending || isLoading || selectedKeys.size === 0}
        >
          {promote.isPending
            ? t('promotions:promote.inProgress', 'Promoting...')
            : t('promotions:promote.confirm', 'Promote {{count}} change', {count: selectedKeys.size})}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/** The diff plus its summary and the deselection hint. */
function PromotionBody({
  diff,
  selectedKeys,
  onToggle,
}: {
  diff: Diff;
  selectedKeys: Set<string>;
  onToggle: (key: string) => void;
}): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <Stack direction="row" spacing={2} alignItems="center" sx={{mb: 2}}>
        <DiffSummaryChips summary={diff.summary} />
        <Typography variant="caption" color="text.secondary">
          {t('promotions:promote.deselectHint', 'Clear a checkbox to leave that change behind.')}
        </Typography>
      </Stack>
      <ResourceDiffList diff={diff} selectable selectedKeys={selectedKeys} onToggle={onToggle} />
    </>
  );
}
