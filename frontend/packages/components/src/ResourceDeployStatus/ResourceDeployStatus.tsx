// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, Chip, Tooltip, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {Rocket} from '@wso2/oxygen-ui-icons-react';
import type {JSX, MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useDeployStatusSource, type DeployStatus} from './DeployStatusContext';

export interface ResourceDeployStatusProps {
  /** The resource's type as the deploy path names it, such as `application` or `flow`. */
  resourceType: string;
  id: string;
  /** A chip for a list row, or a notice with the next step for a resource's own page. */
  variant?: 'chip' | 'notice';
  /** Styles for the notice. */
  sx?: SxProps<Theme>;
}

/**
 * Where a resource is along the deploy path, so a change made here is not taken to be live before it is
 * deployed and promoted. Renders nothing in an app that deploys nothing, and nothing in a list row for
 * a resource every target runs as it is.
 */
export default function ResourceDeployStatus({
  resourceType,
  id,
  variant = 'chip',
  sx = undefined,
}: ResourceDeployStatusProps): JSX.Element | null {
  const {t} = useTranslation('deploy');
  const source = useDeployStatusSource();
  const status: DeployStatus | undefined = source?.statusOf(resourceType, id);
  if (!source || !status) return null;
  const target = status.target ?? '';

  const label: Record<DeployStatus['state'], string> = {
    notDeployed: t('resourceStatus.notDeployed', 'Not deployed'),
    changed: t('resourceStatus.changed', 'Changes not deployed'),
    notPromoted: t('resourceStatus.notPromoted', 'Not in {{target}}', {target}),
    refused: t('resourceStatus.refused', 'Refused by {{target}}', {target}),
    live: t('resourceStatus.live', 'Live'),
  };
  const hint: Record<DeployStatus['state'], string> = {
    notDeployed: t(
      'resourceStatus.notDeployedHint',
      'Saved here only. Deploy it to {{target}} to make it live, then promote it along the path.',
      {target},
    ),
    changed: t(
      'resourceStatus.changedHint',
      'Your latest changes are saved here only. {{target}} still runs the earlier form until you deploy.',
      {target},
    ),
    notPromoted: t(
      'resourceStatus.notPromotedHint',
      'Running on the earlier environments, not yet on {{target}}. Promote it to take it there.',
      {target},
    ),
    refused: status.reason
      ? t('resourceStatus.refusedHint', '{{reason}}. Fix it, then deploy it to {{target}} again.', {
          reason: status.reason.replace(/\.$/, ''),
          target,
        })
      : t('resourceStatus.refusedHintNoReason', 'Fix it, then deploy it to {{target}} again.', {target}),
    live: t('resourceStatus.liveHint', 'Every environment runs it as it is here.'),
  };
  const severity = {
    notDeployed: 'warning',
    changed: 'warning',
    notPromoted: 'info',
    refused: 'error',
    live: 'success',
  } as const;

  if (variant === 'chip') {
    if (status.state === 'live') return null;
    const open = (event: MouseEvent): void => {
      // The row opens the resource on a click; the chip opens the deploy path instead.
      event.stopPropagation();
      source.openDeploy();
    };
    return (
      <Tooltip title={hint[status.state]}>
        <Chip
          size="small"
          variant="outlined"
          color={severity[status.state]}
          icon={<Rocket size={12} />}
          label={label[status.state]}
          onClick={open}
          data-testid="resource-deploy-status"
          sx={{height: 22, fontSize: 11}}
        />
      </Tooltip>
    );
  }

  return (
    <Alert
      severity={severity[status.state]}
      data-testid="resource-deploy-status"
      sx={sx}
      action={
        status.state === 'live' ? undefined : (
          <Button
            color="inherit"
            size="small"
            startIcon={<Rocket size={14} />}
            onClick={() =>
              source.openResource ? source.openResource(resourceType, id, status.target) : source.openDeploy()
            }
            data-testid="resource-deploy-status-action"
          >
            {status.state === 'notPromoted'
              ? t('resourceStatus.promote', 'Promote')
              : t('resourceStatus.deploy', 'Deploy')}
          </Button>
        )
      }
    >
      <strong>
        {label[status.state]}
        {status.state === 'refused' && status.reason ? ':' : ''}
      </strong>{' '}
      {hint[status.state]}
    </Alert>
  );
}
