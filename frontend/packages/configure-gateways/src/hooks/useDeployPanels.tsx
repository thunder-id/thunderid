// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ApplyPanel from '../components/ApplyPanel';
import AutoApplySwitch from '../components/AutoApplySwitch';
import PipelineSteps from '../components/PipelineSteps';
import RevertPanel from '../components/RevertPanel';
import type {AppliedVersion, ApplyResult, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';
import {versionLabel} from '../utils/format';

type Open =
  | {kind: 'apply'; version: Version; gatewayId?: string; only?: string[]; targets?: Gateway[]}
  | {kind: 'revert'; gateway: Gateway}
  | null;

export interface DeployPanels {
  /**
   * Opens a deploy of a version, to a gateway when given, with only the given changes ticked when given.
   * `targets` limits where it can go, every gateway otherwise.
   */
  deploy: (version: Version, gatewayId?: string, only?: string[], targets?: Gateway[]) => void;
  /** Opens a rollback of a gateway. */
  revert: (gateway: Gateway) => void;
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
 * deploy goes, with auto-sync when it goes to the first environment.
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

  const deployed = (gateway: Gateway, result: ApplyResult): void => {
    setOpen(null);
    const failed = result.import?.summary?.failed ?? 0;
    showToast(
      failed > 0
        ? t('applyPanel.doneWithFailures', 'Deployed to {{gateway}}; {{count}} resources were refused.', {
            gateway: gateway.name,
            count: failed,
          })
        : t('applyPanel.done', 'Deployed to {{gateway}}.', {gateway: gateway.name}),
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
    const runs = Object.fromEntries(
      (pipeline ?? []).map((stage: PipelineStage) => [
        stage.gateway.id,
        labelOf(applied[stage.gateway.id]?.appliedVersion),
      ]),
    );
    const first = pipeline?.[0];
    panels = (
      <ApplyPanel
        variant={pipeline ? 'drawer' : 'dialog'}
        before={pipeline ? <PipelineSteps stages={pipeline} targetId={targetId} runs={runs} /> : undefined}
        after={first?.gateway.id === targetId ? <AutoApplySwitch target={first.name} /> : undefined}
        version={open.version}
        label={labelOf(open.version.version)}
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
        previousLabel={labelOf(applied[open.gateway.id]?.previousVersion) || undefined}
        onClose={() => setOpen(null)}
        onReverted={reverted}
      />
    );
  }

  return {
    deploy: (version: Version, gatewayId?: string, only?: string[], targets?: Gateway[]) =>
      setOpen({kind: 'apply', version, gatewayId, only, targets}),
    revert: (gateway: Gateway) => setOpen({kind: 'revert', gateway}),
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
