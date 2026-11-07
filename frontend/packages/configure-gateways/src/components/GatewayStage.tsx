// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, ReactNode} from 'react';
import StageCard from './StageCard';
import useChangedValues from '../api/useChangedValues';
import {useRefused} from '../api/useMaster';
import type {AppliedVersion, MasterEntry, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';

export interface GatewayStageProps {
  gateway: Gateway;
  applied?: AppliedVersion;
  /** The captured versions, newest first, to name the one the gateway runs by its number. */
  versions: Version[];
  onExplore: () => void;
  onConfigure: () => void;
  onRevert: () => void;
  /** Deploys the version the gateway runs again, to take up values set since it was deployed. */
  onRedeploy?: () => void;
  /** Deploys again the resources the gateway refused, given their keys. */
  onRetryRefused?: (keys: string[]) => void;
  /** What the stage is called, the gateway's name otherwise: the gateway it stands for. */
  name?: string;
  /** Opens the gateway's own page. */
  onOpen?: () => void;
  /** Shown in the card's body, such as a setting that applies to this stage. */
  extra?: ReactNode;
  /** The stage's own action, such as deploying or promoting to it. */
  action?: ReactNode;
}

/** One gateway on the deploy path, with the version it runs and when it came to run it. */
export default function GatewayStage({
  gateway,
  applied = undefined,
  versions,
  onExplore,
  onConfigure,
  onRevert,
  onRedeploy = undefined,
  onRetryRefused = undefined,
  name = undefined,
  onOpen = undefined,
  extra = undefined,
  action = undefined,
}: GatewayStageProps): JSX.Element {
  const hash = applied?.appliedVersion ?? undefined;
  const refused = useRefused(gateway.id);
  const changedCheck = useChangedValues(gateway.id, hash);
  const index = hash ? versions.findIndex((version: Version) => version.version === hash) : -1;
  const number = index < 0 ? undefined : (versions[index].number ?? versions.length - index);
  return (
    <StageCard
      name={name ?? gateway.name}
      url={gateway.baseUrl}
      version={hash ? {number, hash, capturedAt: index < 0 ? undefined : versions[index].createdAt} : undefined}
      partial={
        hash && applied?.complete === false
          ? {held: applied.resourcesHeld ?? 0, total: applied.resourcesTotal ?? 0}
          : undefined
      }
      deployedAt={applied?.appliedAt}
      deployedBy={applied?.appliedBy}
      refused={refused}
      onRetryRefused={onRetryRefused ? () => onRetryRefused(refused.map((entry: MasterEntry) => entry.key)) : undefined}
      onExplore={onExplore}
      onConfigure={onConfigure}
      onRevert={onRevert}
      revertDisabled={!applied?.previousVersion}
      onRedeploy={onRedeploy}
      changed={changedCheck.changed}
      changedCheckFailed={changedCheck.failed}
      onRetryChangedCheck={changedCheck.retry}
      onOpen={onOpen}
      extra={extra}
      action={action}
      testId={`deploy-stage-${gateway.id}`}
    />
  );
}
