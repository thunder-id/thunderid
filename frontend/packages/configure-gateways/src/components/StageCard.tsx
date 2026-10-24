// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, ButtonBase, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, RefreshCw, SlidersHorizontal, Undo2} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ActorName from './ActorName';
import CopyableUrl from './CopyableUrl';
import DeployCard from './DeployCard';
import Timestamp from './Timestamp';
import VersionName from './VersionName';
import {changedCount} from '../api/useChangedValues';
import type {Actor, Change, ChangedValues} from '../models/deploy';

export interface StageCardProps {
  name: string;
  url?: string;
  /** The version the stage runs, with when it was captured, or absent while it runs none. */
  version?: {number?: number; hash?: string; capturedAt?: string};
  /** When the stage came to run it, which is not when the version was captured. */
  deployedAt?: string;
  /** Who deployed it, promoted it there or rolled the stage back to it. */
  deployedBy?: Actor;
  /** The changes of its version the stage holds back, so it does not run them. */
  heldBack?: Change[];
  /** Opens the stage's explorer. */
  onExplore: () => void;
  /** Opens the stage's variables and secrets. */
  onConfigure: () => void;
  /** Starts a rollback; absent or disabled when there is nothing to go back to. */
  onRevert?: () => void;
  revertDisabled?: boolean;
  /**
   * Deploys the version the stage runs again, which is how values set since it was deployed take
   * effect. Absent, or while the stage runs nothing, there is no Redeploy.
   */
  onRedeploy?: () => void;
  /** The values set on the stage since its version was deployed. Redeploy is enabled only while any are. */
  changed?: ChangedValues;
  /**
   * The check for changed values failed, so whether any changed is not known. Redeploy then gives way
   * to checking again, rather than saying nothing changed.
   */
  changedCheckFailed?: boolean;
  /** Runs the check for changed values again. */
  onRetryChangedCheck?: () => void;
  /** The stage's own action, at the right of its footer, such as promoting it to the next stage. */
  action?: ReactNode;
  testId?: string;
}

/** One stage of the deploy path: a gateway, or an environment in the cloud. */
export default function StageCard({
  name,
  url = undefined,
  version = undefined,
  deployedAt = undefined,
  deployedBy = undefined,
  heldBack = [],
  onExplore,
  onConfigure,
  onRevert = undefined,
  revertDisabled = false,
  onRedeploy = undefined,
  changed = undefined,
  changedCheckFailed = false,
  onRetryChangedCheck = undefined,
  action = undefined,
  testId = undefined,
}: StageCardProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const owed = changedCount(changed);
  const owedNames = [...(changed?.variables ?? []), ...(changed?.secrets ?? [])].join(', ');
  const header = (
    <ButtonBase
      onClick={onExplore}
      aria-label={t('stage.exploreLabel', 'Explore {{name}}', {name})}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        width: '100%',
        textAlign: 'left',
        px: 2,
        py: 2,
        borderBottom: 1,
        borderColor: 'divider',
        borderRadius: '16px 16px 0 0',
        '&:hover': {bgcolor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.06)'},
      }}
    >
      <Typography variant="subtitle1" fontWeight={600} noWrap sx={{flex: 1, minWidth: 0}}>
        {name}
      </Typography>
      <Box
        sx={{display: 'flex', alignItems: 'center', gap: 0.25, color: 'primary.light', whiteSpace: 'nowrap'}}
        aria-hidden
      >
        <Typography variant="body2" color="inherit">
          {t('stage.explore', 'Explore')}
        </Typography>
        <ArrowRight size={16} />
      </Box>
    </ButtonBase>
  );
  const footer = (
    <>
      <Button variant="text" size="small" startIcon={<SlidersHorizontal size={14} />} onClick={onConfigure}>
        {t('stage.configure', 'Configure')}
      </Button>
      {onRedeploy && version && changedCheckFailed && (
        <Tooltip
          title={t(
            'stage.changedCheckFailed',
            'Could not check whether any value changed since {{name}} was deployed.',
            {name},
          )}
        >
          <Box component="span">
            <Button
              variant="text"
              size="small"
              startIcon={<RefreshCw size={14} />}
              onClick={onRetryChangedCheck}
              disabled={!onRetryChangedCheck}
              data-testid="stage-changed-check-retry"
            >
              {t('stage.checkAgain', 'Check again')}
            </Button>
          </Box>
        </Tooltip>
      )}
      {onRedeploy && version && !changedCheckFailed && (
        <Tooltip
          title={
            owed > 0
              ? t('stage.redeployHint', 'Deploy the version {{name}} runs again, so {{values}} take effect.', {
                  name,
                  values: owedNames,
                })
              : t('stage.redeployNone', 'Nothing to redeploy: no value has changed since {{name}} was deployed.', {
                  name,
                })
          }
        >
          {/* A disabled button takes no hover, so the hint is held by what wraps it. */}
          <Box component="span">
            <Button
              variant="text"
              size="small"
              startIcon={<RefreshCw size={14} />}
              onClick={onRedeploy}
              disabled={owed === 0}
              data-testid="stage-redeploy"
            >
              {t('stage.redeploy', 'Redeploy')}
            </Button>
          </Box>
        </Tooltip>
      )}
      {onRevert && (
        <Tooltip
          title={
            revertDisabled
              ? t(
                  'stage.revertNone',
                  'Nothing to roll back to. A rollback goes back one step, once; deploy a version to {{name}} to roll back again.',
                  {name},
                )
              : t('stage.revertHint', 'Go back one step, to the version {{name}} ran before.', {name})
          }
        >
          {/* A disabled button takes no hover, so the hint is held by what wraps it. */}
          <Box component="span">
            <Button
              variant="text"
              color="error"
              size="small"
              startIcon={<Undo2 size={14} />}
              onClick={onRevert}
              disabled={revertDisabled}
            >
              {t('stage.revert', 'Rollback')}
            </Button>
          </Box>
        </Tooltip>
      )}
      {action && <Box sx={{ml: 'auto', display: 'flex', gap: 1}}>{action}</Box>}
    </>
  );
  return (
    <DeployCard header={header} footer={footer} testId={testId}>
      <Stack spacing={2} sx={{px: 2, py: 2, flex: 1}}>
        <Box>
          <Typography variant="caption" color="text.secondary" component="div">
            {t('stage.deployedVersion', 'Deployed version')}
          </Typography>
          {version ? (
            <Stack spacing={0.5} sx={{mt: 0.25}}>
              <VersionName number={version.number} hash={version.hash} variant="h6" />
              <Timestamp value={version.capturedAt} />
            </Stack>
          ) : (
            <Typography variant="h6" fontWeight={600}>
              {t('stage.nothingDeployed', 'Nothing deployed yet')}
            </Typography>
          )}
        </Box>
        {version && (
          <Box sx={{borderTop: 1, borderColor: 'divider', pt: 1.5}} data-testid="stage-deployed">
            <Typography variant="caption" color="text.secondary" component="div">
              {t('stage.deployedAt', 'Deployed')}
            </Typography>
            <Stack spacing={0.75} sx={{mt: 0.5}}>
              {deployedAt ? (
                <Timestamp value={deployedAt} />
              ) : (
                <Typography variant="body2" color="text.secondary">
                  {t('stage.deployedUnknown', 'Not recorded')}
                </Typography>
              )}
              {deployedBy && <ActorName actor={deployedBy} />}
              {owed > 0 && (
                <Tooltip title={owedNames}>
                  <Typography
                    variant="caption"
                    color="warning.main"
                    data-testid="stage-changed"
                    sx={{
                      width: 'fit-content',
                      cursor: 'help',
                      borderBottom: 1,
                      borderStyle: 'dotted',
                      borderColor: 'divider',
                    }}
                  >
                    {t('stage.changed', '{{count}} values changed since deploy', {count: owed})}
                  </Typography>
                </Tooltip>
              )}
              {heldBack.length > 0 && (
                <Tooltip title={heldBack.map((change: Change) => change.name ?? change.id ?? change.key).join(', ')}>
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    data-testid="stage-held-back"
                    sx={{
                      width: 'fit-content',
                      cursor: 'help',
                      borderBottom: 1,
                      borderStyle: 'dotted',
                      borderColor: 'divider',
                    }}
                  >
                    {t('stage.heldBack', '{{count}} held back, not running', {count: heldBack.length})}
                  </Typography>
                </Tooltip>
              )}
            </Stack>
          </Box>
        )}
        {url && (
          <Box sx={{mt: 'auto'}}>
            <Typography variant="caption" color="text.secondary" component="div" sx={{mb: 0.5}}>
              {t('stage.url', 'URL')}
            </Typography>
            <CopyableUrl url={url} />
          </Box>
        )}
      </Stack>
    </DeployCard>
  );
}
