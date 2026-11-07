// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {CircleAlert, CircleCheck, Compass, Info, RefreshCw, Undo2} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useChangedValues, {changedCount} from '../api/useChangedValues';
import {useApplyCheck} from '../api/useGatewayVersions';
import {useRefused} from '../api/useMaster';
import {gatewayColor} from '../constants/gateway-colors';
import useActorName from '../hooks/useActorName';
import type {AppliedVersion, Change, MasterEntry, Version} from '../models/deploy';
import type {PipelineStage} from '../models/pipeline';
import type {ProjectStatus} from '../models/projectStatus';
import {versionLabel} from '../utils/format';
import relativeTime from '../utils/relativeTime';
import {projectRollbackOf, rollbackHint} from '../utils/rollback';

/** What the card says about a gateway against what it is compared with. */
export type GatewayStanding =
  /** The first gateway runs the latest version whole. */
  | {kind: 'inSync'}
  /** The first gateway runs less than the latest version; how many changes, when known. */
  | {kind: 'behind'; changes?: number}
  /** A later gateway runs what the one before it runs. */
  | {kind: 'matches'; previous: string}
  /** A later gateway runs a version this many versions older than the one before it. */
  | {kind: 'versionsBehind'; previous: string; count: number}
  /** A later gateway runs something else, or nothing, while the one before runs a version. */
  | {kind: 'ready'; previous: string}
  /** A project's own changes, and what they need, wait to be promoted there from the one before it. */
  | {kind: 'projectBehindPrevious'; count: number; previous: string};

export interface GatewayCardProps {
  stage: PipelineStage;
  /** Its place in the pipeline: the first is where a deploy enters. */
  index: number;
  applied?: AppliedVersion;
  /** The captured versions, newest first, to name the ones the gateway runs and ran. */
  versions: Version[];
  standing?: GatewayStanding;
  /**
   * How the selected project stands there. Given, the card speaks of the project's own resources, not of
   * the version the gateway last received, since a promotion can carry part of a version.
   */
  projectStatus?: ProjectStatus;
  /** On a project's home: the card speaks of the project alone, and never of the whole gateway. */
  inProject?: boolean;
  /** On a project's home, the project's status there could not be read. */
  statusFailed?: boolean;
  /** Reads the project's status there again. */
  onRetryStatus?: () => void;
  /** The gateway before it, which the project's needed resources are compared with. */
  previousName?: string;
  /** Opens the deploy or promotion that would bring what is waiting, from the needed resources line. */
  onReview?: () => void;
  onExplore: () => void;
  onRevert: () => void;
  /** Opens the gateway's own page. */
  onOpen?: () => void;
  /** Promotes what the gateway runs to the next gateway, offered while that one runs something else. */
  onPromote?: () => void;
  /** Shown above the actions, such as the auto-sync switch. */
  extra?: ReactNode;
}

/** A line under the deployed line that says how the gateway stands, in a color for its state. */
function Highlight({
  tone,
  icon,
  children,
  testId,
}: {
  tone: 'warning' | 'error' | 'neutral' | 'info';
  icon: ReactNode;
  children: ReactNode;
  testId: string;
}): JSX.Element {
  // Information reads in the muted text of the card, with only its icon in a soft accent.
  const color = {warning: 'warning.main', error: 'error.main', neutral: 'text.secondary', info: 'text.secondary'}[tone];
  return (
    <Stack direction="row" spacing={0.75} alignItems="center" sx={{color}} data-testid={testId}>
      {tone === 'info' ? (
        <Box component="span" sx={{display: 'flex', color: 'info.light', opacity: 0.8}}>
          {icon}
        </Box>
      ) : (
        icon
      )}
      <Typography variant="body2" color="inherit">
        {children}
      </Typography>
    </Stack>
  );
}

/** Who deployed the version: a name, or nothing when the deploy recorded no one. */
function DeployedBy({
  at,
  by,
  promoted,
  rolledBack = false,
}: {
  at: string | undefined;
  by: AppliedVersion['appliedBy'];
  promoted: boolean;
  rolledBack?: boolean;
}): JSX.Element {
  const {t} = useTranslation('deploy');
  const actor = useActorName(by ?? {id: ''});
  const when = relativeTime(at);
  let text: string;
  if (rolledBack) {
    text = by
      ? t('card.rolledBackBy', 'Rolled back {{when}} by {{actor}}', {when, actor})
      : t('card.rolledBack', 'Rolled back {{when}}', {when});
  } else if (promoted) {
    text = by
      ? t('card.promotedBy', 'Promoted {{when}} by {{actor}}', {when, actor})
      : t('card.promoted', 'Promoted {{when}}', {when});
  } else {
    text = by
      ? t('card.deployedBy', 'Deployed {{when}} by {{actor}}', {when, actor})
      : t('card.deployed', 'Deployed {{when}}', {when});
  }
  return (
    <Typography variant="body2" color="text.secondary" data-testid="card-deployed">
      {text}
    </Typography>
  );
}

/**
 * One gateway along the pipeline, as the console home shows it: marked in its color, with the
 * version it runs, who brought it there and when, how it stands against the latest version or the
 * gateway before it, and what can be done with it.
 */
export default function GatewayCard({
  stage,
  index,
  applied = undefined,
  versions,
  standing = undefined,
  projectStatus = undefined,
  inProject = false,
  statusFailed = false,
  onRetryStatus = undefined,
  previousName = undefined,
  onReview = undefined,
  onExplore,
  onRevert,
  onOpen = undefined,
  onPromote = undefined,
  extra = undefined,
}: GatewayCardProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const {gateway} = stage;
  const color = gatewayColor(index);
  const hash = applied?.appliedVersion ?? undefined;
  const scoped = inProject || Boolean(projectStatus);
  const wholeRefused = useRefused(gateway.id);
  // In a project, what the gateway refused is read from the project's check of what it runs, which
  // places each refused change in the project's sections: the project's own and what it cannot run
  // without count, and refused shared resources are said apart.
  const projectCheck = useApplyCheck(gateway.id, hash, Boolean(projectStatus) && Boolean(hash));
  const projectFailed = (projectCheck.data?.diff.changes ?? []).filter((change: Change) => change.failed);
  const isOwn = (change: Change): boolean => change.section === 'project' || change.required === true;
  const nameOfChange = (change: Change): string => change.name ?? change.id ?? change.key;
  const refused: {key: string; label: string}[] = scoped
    ? projectFailed.filter(isOwn).map((change: Change) => ({key: change.key, label: nameOfChange(change)}))
    : wholeRefused.map((entry: MasterEntry) => ({key: entry.key, label: entry.name ?? entry.id ?? entry.key}));
  const sharedRefused = scoped
    ? projectFailed.filter((change: Change) => !isOwn(change) && change.section !== 'other')
    : [];
  const changedCheck = useChangedValues(gateway.id, hash);
  const owed = changedCount(changedCheck.changed);
  const labelOf = (ref: string | undefined): string | undefined => {
    if (!ref) return undefined;
    const found = versions.find((version: Version) => version.version === ref);
    return versionLabel(ref, found?.number);
  };
  const projectVersions = projectStatus?.project.versions ?? [];
  const projectLabel = (share: {hash: string; number?: number}): string =>
    versionLabel(
      share.hash,
      share.number ?? versions.find((version: Version) => version.version === share.hash)?.number,
    );
  // On a project's home the card names the version the project's own resources came from: the
  // gateway can hold another project's newer resources, or only part of a version.
  const running = scoped ? projectVersions[0] && projectLabel(projectVersions[0]) : labelOf(hash);
  const rollback = scoped ? projectRollbackOf(projectStatus) : undefined;
  const previous = scoped ? rollback && projectLabel(rollback.to) : labelOf(applied?.previousVersion);
  const rollbackLabel = rollback?.undo
    ? t('card.undoLastDeploy', 'Undo last deploy')
    : previous
      ? t('card.rollBackTo', 'Roll back to {{version}}', {version: previous})
      : t('stage.revert', 'Rollback');
  const sharedTypes = (projectStatus?.shared.byType ?? [])
    .map(({resourceType, count}: {resourceType: string; count: number}) =>
      t('card.sharedType', '{{count}} {{type}}', {count, type: resourceType}),
    )
    .join(', ');

  let highlight: JSX.Element | null = null;
  switch (standing?.kind) {
    case 'inSync':
      highlight = (
        <Highlight tone="neutral" icon={<CircleCheck size={14} />} testId="card-standing">
          {t('card.inSync', 'In sync with configuration')}
        </Highlight>
      );
      break;
    case 'behind':
      highlight = (
        <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-standing">
          {standing.changes
            ? t('card.changesBehind', {
                count: standing.changes,
                defaultValue_one: '{{count}} change behind',
                defaultValue_other: '{{count}} changes behind',
              })
            : t('card.newerReady', 'Newer version ready')}
        </Highlight>
      );
      break;
    case 'matches':
      highlight = (
        <Highlight tone="neutral" icon={<CircleCheck size={14} />} testId="card-standing">
          {t('card.matches', 'Matches {{previous}}', {previous: standing.previous})}
        </Highlight>
      );
      break;
    case 'versionsBehind':
      highlight = (
        <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-standing">
          {t('card.versionsBehind', {
            count: standing.count,
            previous: standing.previous,
            defaultValue_one: '{{count}} version behind {{previous}}',
            defaultValue_other: '{{count}} versions behind {{previous}}',
          })}
        </Highlight>
      );
      break;
    case 'ready':
      highlight = (
        <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-standing">
          {t('card.readyFrom', 'Ready to promote from {{previous}}', {previous: standing.previous})}
        </Highlight>
      );
      break;
    case 'projectBehindPrevious':
      highlight = (
        <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-standing">
          {t('card.projectBehindPrevious', '{{count}} behind {{previous}}', {
            count: standing.count,
            previous: standing.previous,
          })}
        </Highlight>
      );
      break;
    default:
      break;
  }

  // A project can run on more than one of its versions there, when a promotion carried part of one.
  const partly = projectVersions.length > 1 ? projectVersions.slice(1).map(projectLabel).join(', ') : undefined;

  return (
    <Stack
      spacing={1.5}
      data-testid={`deploy-stage-${gateway.id}`}
      sx={(theme) => ({
        flex: '0 0 260px',
        scrollSnapAlign: 'start',
        p: 2.5,
        borderRadius: '12px',
        border: 1,
        borderColor: 'divider',
        borderTop: `3px solid ${color.light}`,
        bgcolor: 'background.paper',
        ...theme.applyStyles('dark', {borderTopColor: color.dark}),
      })}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Typography
          noWrap
          sx={(theme) => ({
            flex: 1,
            minWidth: 0,
            fontFamily: 'monospace',
            fontSize: 13,
            fontWeight: 500,
            color: color.light,
            ...theme.applyStyles('dark', {color: color.dark}),
          })}
        >
          {stage.name.toUpperCase()}
        </Typography>
        {running && (
          <Typography sx={{fontFamily: 'monospace', fontSize: 12}} color="text.secondary" data-testid="card-version">
            {running}
          </Typography>
        )}
        <Tooltip title={t('card.explore', 'Explore')}>
          <IconButton
            size="small"
            onClick={onExplore}
            aria-label={t('stage.exploreLabel', 'Explore {{name}}', {name: stage.name})}
          >
            <Compass size={16} />
          </IconButton>
        </Tooltip>
      </Stack>
      <Typography variant="h6" component="h3" fontWeight={500} noWrap>
        {stage.name}
      </Typography>
      {/* The status, as tall as four lines on every card, so the open button lines up across the pipeline. */}
      <Stack spacing={1.5} sx={{minHeight: 116}}>
        {scoped &&
          (projectStatus && projectVersions.length > 0 ? (
            <DeployedBy
              at={projectStatus.deployedAt}
              by={projectStatus.deployedBy}
              promoted={index > 0}
              rolledBack={projectStatus.lastAction === 'rolledBack'}
            />
          ) : projectStatus ? (
            <Typography variant="body2" color="text.secondary" data-testid="card-deployed">
              {t('card.noProjectResources', 'No project resources yet')}
            </Typography>
          ) : null)}
        {!scoped &&
          (hash && applied ? (
            <DeployedBy at={applied.appliedAt} by={applied.appliedBy} promoted={index > 0} />
          ) : (
            <Typography variant="body2" color="text.secondary" data-testid="card-deployed">
              {t('stage.nothingDeployed', 'Nothing deployed yet')}
            </Typography>
          ))}
        {scoped && !projectStatus && statusFailed && (
          <Stack direction="row" alignItems="center" spacing={1} sx={{flexWrap: 'wrap'}}>
            <Highlight tone="error" icon={<CircleAlert size={14} />} testId="card-status-failed">
              {t('card.statusFailed', "This project's status here could not be read.")}
            </Highlight>
            {onRetryStatus && (
              <Button
                variant="text"
                size="small"
                startIcon={<RefreshCw size={14} />}
                onClick={onRetryStatus}
                data-testid="card-status-retry"
                sx={{py: 0, minHeight: 0}}
              >
                {t('card.retry', 'Retry')}
              </Button>
            )}
          </Stack>
        )}
        {highlight}
        {partly && (
          <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-partly">
            {t('card.projectPartly', 'Partly on {{versions}}', {versions: partly})}
          </Highlight>
        )}
        {projectStatus?.autoSync?.lastRun?.error && (
          <Stack direction="row" alignItems="center" spacing={1} sx={{flexWrap: 'wrap'}}>
            <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="card-auto-sync-failed">
              {t('card.autoSyncFailed', 'Auto-sync failed: {{message}}', {
                message: projectStatus.autoSync.lastRun.error,
              })}
            </Highlight>
            {onReview && (
              <Button
                variant="text"
                size="small"
                startIcon={<RefreshCw size={14} />}
                onClick={onReview}
                data-testid="card-auto-sync-retry"
                sx={{py: 0, minHeight: 0}}
              >
                {t('card.retry', 'Retry')}
              </Button>
            )}
          </Stack>
        )}
        {projectStatus && projectStatus.shared.pending > 0 && (
          <Tooltip
            title={t(
              'card.sharedHint',
              "{{types}}. Shared resources belong to no project: they go with any project's promotion when ticked.",
              {types: sharedTypes},
            )}
          >
            <Box component="span" sx={{width: 'fit-content', cursor: 'help'}}>
              <Highlight tone="info" icon={<Info size={14} />} testId="card-shared">
                {t('card.sharedAvailable', {
                  count: projectStatus.shared.pending,
                  defaultValue_one: '{{count}} shared update available',
                  defaultValue_other: '{{count}} shared updates available',
                })}
              </Highlight>
            </Box>
          </Tooltip>
        )}
        {projectStatus && projectStatus.needed.differing > 0 && previousName && (
          <Button
            variant="text"
            size="small"
            color="inherit"
            onClick={onReview}
            disabled={!onReview}
            sx={{alignSelf: 'flex-start', px: 0, minHeight: 0, color: 'text.secondary', textAlign: 'left'}}
            data-testid="card-needed-differing"
          >
            {t('card.neededDiffering', {
              count: projectStatus.needed.differing,
              previous: previousName,
              defaultValue_one: 'Uses {{count}} needed resource in another form than {{previous}}',
              defaultValue_other: 'Uses {{count}} needed resources in another form than {{previous}}',
            })}
          </Button>
        )}
        {!scoped && hash && applied?.complete === false && (
          <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="stage-partial">
            {t('stage.partlyApplied', '{{version}} partly applied: {{held}} of {{total}} resources', {
              version: running,
              held: applied.resourcesHeld ?? 0,
              total: applied.resourcesTotal ?? 0,
            })}
          </Highlight>
        )}
        {refused.length > 0 && (
          <Tooltip title={refused.map((entry) => entry.label).join(', ')}>
            <Box component="span" sx={{width: 'fit-content', cursor: 'help'}}>
              <Highlight tone="error" icon={<CircleAlert size={14} />} testId="stage-refused">
                {t('stage.refused', '{{count}} refused, not running', {count: refused.length})}
              </Highlight>
            </Box>
          </Tooltip>
        )}
        {sharedRefused.length > 0 && (
          <Tooltip title={sharedRefused.map(nameOfChange).join(', ')}>
            <Box component="span" sx={{width: 'fit-content', cursor: 'help'}}>
              <Highlight tone="neutral" icon={<CircleAlert size={14} />} testId="card-shared-refused">
                {t('card.sharedRefused', {
                  count: sharedRefused.length,
                  defaultValue_one: '{{count}} shared resource refused',
                  defaultValue_other: '{{count}} shared resources refused',
                })}
              </Highlight>
            </Box>
          </Tooltip>
        )}
        {owed > 0 && (
          <Highlight tone="warning" icon={<CircleAlert size={14} />} testId="stage-changed">
            {t('stage.changed', '{{count}} values changed since deploy', {count: owed})}
          </Highlight>
        )}
        {extra && (
          <Box
            sx={(theme) => ({
              px: 1.25,
              py: 0.5,
              borderRadius: '8px',
              bgcolor: 'action.hover',
              ...theme.applyStyles('dark', {bgcolor: 'rgba(0, 0, 0, 0.35)'}),
            })}
          >
            {extra}
          </Box>
        )}
      </Stack>
      {onOpen && (
        <Button
          variant="outlined"
          size="small"
          onClick={onOpen}
          sx={{borderRadius: 100, alignSelf: 'center', whiteSpace: 'nowrap'}}
          data-testid="stage-open"
        >
          {t('stage.open', 'Open gateway')}
        </Button>
      )}
      <Box sx={{flex: 1}} />
      {/* Rollback and promote keep to one row, promote at the end: a long rollback label gives way first. */}
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{flexWrap: 'nowrap', minWidth: 0}}
        data-testid="card-actions"
      >
        <Tooltip
          title={
            scoped
              ? rollback
                ? rollbackHint(t, rollback, previous ?? '', stage.name)
                : t('card.projectRevertNone', 'Nothing to roll back to for this project on {{name}}.', {
                    name: stage.name,
                  })
              : previous
                ? t('stage.revertHint', 'Go back one step, to the version {{name}} ran before.', {name: stage.name})
                : t(
                    'stage.revertNone',
                    'Nothing to roll back to. A rollback goes back one step, once; deploy a version to {{name}} to roll back again.',
                    {name: stage.name},
                  )
          }
        >
          {/* A disabled button takes no hover, so the hint is held by what wraps it. */}
          <Box component="span" sx={{display: 'flex', minWidth: 0, flex: '0 1 auto'}}>
            <Button
              variant="text"
              size="small"
              color="inherit"
              startIcon={<Undo2 size={14} />}
              onClick={onRevert}
              disabled={!previous}
              sx={{color: 'text.secondary', minWidth: 0, maxWidth: '100%'}}
              data-testid="card-revert"
            >
              <Box component="span" sx={{overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}>
                {rollbackLabel}
              </Box>
            </Button>
          </Box>
        </Tooltip>
        <Box sx={{flex: 1}} />
        {onPromote && (
          <Button
            variant="contained"
            size="small"
            onClick={onPromote}
            sx={{flexShrink: 0, whiteSpace: 'nowrap', minWidth: 0, borderRadius: 100}}
            data-testid="card-promote"
          >
            {t('gateways.promote', 'Promote')}
          </Button>
        )}
      </Stack>
    </Stack>
  );
}
