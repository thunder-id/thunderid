// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo, type JSX} from 'react';
import useDeployPanels from './useDeployPanels';
import useAppliedVersions from '../api/useAppliedVersions';
import {usePipeline} from '../api/usePipeline';
import {useVersions} from '../api/useVersions';
import type {Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';

export interface ResourceDeploy {
  /**
   * Opens the deploy, or the promotion, that brings one resource to a gateway, named as the pipeline
   * names it, with that resource alone ticked; the first gateway when none is named.
   */
  open: (resourceType: string, id: string, target?: string) => void;
  /** The drawer, while one is open. */
  panels: JSX.Element | null;
}

/**
 * Brings one resource along the pipeline on its own, such as from its page: a deploy of the latest version
 * to the first gateway, or a promotion of what the gateway before runs to a later one.
 */
export default function useResourceDeploy(): ResourceDeploy {
  const pipeline = usePipeline();
  const versions = useVersions();
  const stages: PipelineStage[] = useMemo(() => pipeline.data?.stages ?? [], [pipeline.data]);
  const gateways: Gateway[] = useMemo(() => stages.map((stage: PipelineStage) => stage.gateway), [stages]);
  const applied = useAppliedVersions(gateways.map((gateway: Gateway) => gateway.id));
  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  const panels = useDeployPanels(gateways, all, applied.byGateway, stages);

  const open = (resourceType: string, id: string, target?: string): void => {
    const at = Math.max(
      0,
      stages.findIndex((stage: PipelineStage) => stage.name === target),
    );
    const stage = stages[at];
    if (!stage) return;
    const from = at > 0 ? applied.byGateway[stages[at - 1].gateway.id]?.appliedVersion : undefined;
    const version = at > 0 ? all.find((entry: Version) => entry.version === from) : all[0];
    if (!version) return;
    panels.deploy(version, stage.gateway.id, [`${resourceType}/${id}`], [stage.gateway]);
  };

  return {open, panels: panels.panels};
}
