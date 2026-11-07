// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useProjectStatuses} from '../api/useProjectStatus';
import ApplyPanel from '../components/ApplyPanel';
import AutoApplySwitch from '../components/AutoApplySwitch';
import PipelineSteps from '../components/PipelineSteps';
import ResourceNamesProvider from '../components/ResourceNamesProvider';
import RevertPanel from '../components/RevertPanel';
import useProjectScope from '../contexts/ProjectScope';
import type {AppliedVersion, ApplyResult, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';
import {versionLabel} from '../utils/format';

/** No gateways, for when there is no project to read the status of. */
const NO_STAGES: PipelineStage[] = [];

type Open =
  | {kind: 'apply'; version: Version; gatewayId?: string; only?: string[]; targets?: Gateway[]}
  | {kind: 'revert'; gateway: Gateway; label?: string; undo?: boolean}
  | null;

export interface DeployPanels {
  /**
   * Opens a deploy of a version, to a gateway when given, with only the given changes ticked when given.
   * `targets` limits where it can go, every gateway otherwise.
   */
  deploy: (version: Version, gatewayId?: string, only?: string[], targets?: Gateway[]) => void;
  /** Opens a rollback of a gateway. */
  /** Rolls a gateway back one step, naming the version it goes back to when the caller knows it. */
  revert: (gateway: Gateway, label?: string, undo?: boolean) => void;
  /** Opens a deploy of the version a gateway runs again, so the values set on it since take effect. */
  redeploy: (gateway: Gateway) => void;
  /** Opens a deploy of the version a gateway runs with only what it refused ticked. */
  retryRefused: (gateway: Gateway, keys: string[]) => void;
  /** The version being deployed, while a deploy is open. */
  deploying?: string;
  /** The deploy or rollback dialog that is open, if any. */
  panels: JSX.Element | null;
  /** The label of a version by its hash, "v6", or empty when it is not one of the versions. */
  labelOf: (hash: string | undefined) => string;
}

/**
 * The deploy and rollback dialogs of the deploy path, opened from its versions and stages, and the toast
 * each says it is done with. Given the pipeline, a deploy opens in a drawer that shows where along it the
 * deploy goes, with auto-sync when it goes to the first gateway.
 */
export default function useDeployPanels(
  gateways: Gateway[],
  versions: Version[],
  applied: Record<string, AppliedVersion | undefined>,
  pipeline?: PipelineStage[],
): DeployPanels {
  const {t} = useTranslation('deploy');
  const {showToast} = useToast();
  const [open, setOpen] = useState<Open>(null);
  const project = useProjectScope();
  const statuses = useProjectStatuses(project && pipeline ? pipeline : NO_STAGES);

  const labelOf = useCallback(
    (hash: string | undefined): string => {
      const index = versions.findIndex((version: Version) => version.version === hash);
      return index < 0
        ? ''
        : versionLabel(versions[index].hash ?? hash, versions[index].number ?? versions.length - index);
    },
    [versions],
  );
  const running = (gateway: Gateway): Version | undefined =>
    versions.find((version: Version) => version.version === applied[gateway.id]?.appliedVersion);

  // What a gateway is called, falling back to its gateway's name off the pipeline.
  const nameOf = (gateway: Gateway): string =>
    pipeline?.find((stage: PipelineStage) => stage.gateway.id === gateway.id)?.name ?? gateway.name;
  const deployed = (gateway: Gateway, result: ApplyResult): void => {
    setOpen(null);
    const failed = result.import?.summary?.failed ?? 0;
    showToast(
      failed > 0
        ? t('applyPanel.doneWithFailures', 'Deployed to {{gateway}}; {{count}} resources were refused.', {
            gateway: nameOf(gateway),
            count: failed,
          })
        : t('applyPanel.done', 'Deployed to {{gateway}}.', {gateway: nameOf(gateway)}),
      failed > 0 ? 'warning' : 'success',
    );
  };
  const reverted = (gateway: Gateway, result: ApplyResult): void => {
    setOpen(null);
    const failed = result.import?.summary?.failed ?? 0;
    showToast(
      failed > 0
        ? t(
            'revertPanel.doneWithFailures',
            '{{gateway}} runs its earlier version again; {{count}} resources were refused.',
            {
              gateway: gateway.name,
              count: failed,
            },
          )
        : t('revertPanel.done', '{{gateway}} runs its earlier version again.', {gateway: gateway.name}),
      failed > 0 ? 'warning' : 'success',
    );
  };

  let panels: JSX.Element | null = null;
  if (open?.kind === 'apply') {
    const targetId = open.gatewayId ?? (open.targets ?? gateways)[0]?.id ?? '';
    // In a project, each gateway runs the version the project's own resources there came from.
    const runs = Object.fromEntries(
      (pipeline ?? []).map((stage: PipelineStage) => {
        if (project) {
          const share = statuses.byGateway[stage.gateway.id]?.project.versions[0];
          return [stage.gateway.id, share ? versionLabel(share.hash, share.number) : ''];
        }
        return [stage.gateway.id, labelOf(applied[stage.gateway.id]?.appliedVersion)];
      }),
    );
    const first = pipeline?.[0];
    const at = (pipeline ?? []).findIndex((stage: PipelineStage) => stage.gateway.id === targetId);
    const stage = at < 0 ? undefined : pipeline?.[at];
    // A later gateway given a version it does not run yet is promoted to, from the one before it; the
    // version it runs again, to redeploy it or retry what it refused, is deployed to it as any deploy.
    const promotedFrom =
      at > 0 && applied[targetId]?.appliedVersion !== open.version.version ? pipeline?.[at - 1] : undefined;
    const label = labelOf(open.version.version);
    let subtitle: string | undefined;
    if (promotedFrom && stage)
      subtitle = t('applyPanel.promoteSubtitle', 'Carries {{version}} from {{source}} to {{target}}.', {
        version: label,
        source: promotedFrom.name,
        target: stage.name,
      });
    else if (first)
      subtitle = t(
        'applyPanel.deploySubtitle',
        'Every deploy enters at {{first}} and moves forward one stage at a time.',
        {
          first: first.name,
        },
      );
    panels = (
      <ApplyPanel
        variant={pipeline ? 'drawer' : 'dialog'}
        before={pipeline ? <PipelineSteps stages={pipeline} targetId={targetId} runs={runs} /> : undefined}
        after={
          first?.gateway.id === targetId ? (
            <AutoApplySwitch target={first.name} targetId={first.gateway.id} />
          ) : undefined
        }
        targetName={stage?.name}
        sourceName={promotedFrom?.name}
        subtitle={subtitle}
        version={open.version}
        label={label}
        gateways={open.targets ?? gateways}
        gatewayId={open.gatewayId}
        only={open.only}
        onClose={() => setOpen(null)}
        onDeployed={deployed}
      />
    );
  } else if (open?.kind === 'revert') {
    panels = (
      <RevertPanel
        gateway={open.gateway}
        previousLabel={open.label ?? (labelOf(applied[open.gateway.id]?.previousVersion) || undefined)}
        onClose={() => setOpen(null)}
        onReverted={reverted}
        undo={open.undo}
        targetName={nameOf(open.gateway)}
      />
    );
  }

  if (panels) {
    panels = (
      <ResourceNamesProvider
        targetIds={(pipeline?.map((stage: PipelineStage) => stage.gateway) ?? gateways).map(
          (gateway: Gateway) => gateway.id,
        )}
      >
        {panels}
      </ResourceNamesProvider>
    );
  }

  return {
    deploy: (version: Version, gatewayId?: string, only?: string[], targets?: Gateway[]) =>
      setOpen({kind: 'apply', version, gatewayId, only, targets}),
    revert: (gateway: Gateway, label?: string, undo?: boolean) => setOpen({kind: 'revert', gateway, label, undo}),
    redeploy: (gateway: Gateway) => {
      const version = running(gateway);
      if (version) setOpen({kind: 'apply', version, gatewayId: gateway.id});
    },
    retryRefused: (gateway: Gateway, keys: string[]) => {
      const version = running(gateway);
      if (version) setOpen({kind: 'apply', version, gatewayId: gateway.id, only: keys});
    },
    deploying: open?.kind === 'apply' ? open.version.version : undefined,
    panels,
    labelOf,
  };
}
