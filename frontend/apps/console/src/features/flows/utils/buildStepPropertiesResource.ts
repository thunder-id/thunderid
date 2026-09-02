// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {ResourceTypes, type Resource} from '../models/resources';
import {StepCategories, StepTypes, type Step, type StepData} from '../models/steps';

/**
 * Builds the resource handed to `setLastInteractedResource` when opening a
 * step's properties from outside the node component (e.g. the canvas context
 * menu). Mirrors the resource each node type constructs for its own configure
 * action, so the properties panel renders identically either way.
 *
 * @param node - The canvas node whose properties should open.
 * @param stepResources - The step resources catalog (used by CALL steps, which
 *   derive their panel resource from their palette entry).
 * @returns The resource to select, or `null` when the node type has no
 *   properties panel.
 */
export default function buildStepPropertiesResource(node: Node, stepResources: Resource[]): Resource | null {
  const data = node.data as StepData | undefined;

  if (node.type === StepTypes.Execution) {
    const executorName = data?.action?.executor?.name ?? 'Executor';
    const displayFromData = data?.display as
      | {
          label?: string;
          image?: string;
          preserveImageColor?: boolean;
          description?: string;
          outcomes?: {success?: string; failure?: string; incomplete?: string};
        }
      | undefined;

    return {
      id: node.id,
      type: 'EXECUTION',
      category: StepCategories.Workflow,
      resourceType: ResourceTypes.Step,
      data,
      display: {
        label: displayFromData?.label ?? executorName,
        image: displayFromData?.image ?? '',
        preserveImageColor: displayFromData?.preserveImageColor,
        description: displayFromData?.description,
        showOnResourcePanel: false,
        outcomes: displayFromData?.outcomes,
      },
    } as Step;
  }

  if (node.type === StepTypes.Rule) {
    return {
      ...(typeof data === 'object' && data !== null ? data : {}),
      id: node.id,
    } as Resource;
  }

  if (node.type === StepTypes.Call) {
    const paletteEntry = stepResources.find((step: Resource) => step.type === StepTypes.Call) as Step | undefined;

    return {
      ...(paletteEntry ?? ({} as Step)),
      id: node.id,
      type: StepTypes.Call,
      category: StepCategories.Workflow,
      resourceType: ResourceTypes.Step,
      data: data ?? {},
      display: {
        ...(paletteEntry?.display ?? {}),
        label: paletteEntry?.display?.label ?? 'Flow',
        showOnResourcePanel: false,
      },
    } as Step;
  }

  return null;
}
