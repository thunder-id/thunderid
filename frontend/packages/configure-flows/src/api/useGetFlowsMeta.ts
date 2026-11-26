// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import actions from '../data/actions.json';
import elements from '../data/elements.json';
import steps from '../data/steps.json';
import rawTemplates from '../data/templates.json';
import widgets from '../data/widgets.json';
import type {FlowType} from '../models/flows';
import type {FlowTemplate} from '../models/templates';

export interface FlowsMetaOptions {
  flowType?: FlowType;
}

export interface FlowsMeta {
  templates: FlowTemplate[];
  steps: unknown[];
  actions: unknown[];
  elements: unknown[];
  widgets: unknown[];
  executors: unknown[];
}

/**
 * Hook to get flow meta resources (templates, steps, actions, elements, widgets, executors).
 *
 * Templates are filtered by `flowType` when provided. Other resources are returned as-is.
 *
 * TODO: Replace local data files with a REST API call (GET /flows/meta?flowType=...) when
 * the endpoint is available.
 *
 * @param options - Optional filter options
 * @returns FlowsMeta object containing filtered templates and all other resource lists
 */
const useGetFlowsMeta = (options?: FlowsMetaOptions): {data: FlowsMeta; error: null; isLoading: false} => {
  const templates = useMemo<FlowTemplate[]>(() => {
    const all = rawTemplates as FlowTemplate[];
    return options?.flowType ? all.filter((t) => t.flowType === options.flowType) : all;
  }, [options?.flowType]);

  const data = useMemo<FlowsMeta>(
    () => ({
      actions,
      elements,
      executors: [],
      steps,
      templates,
      widgets,
    }),
    [templates],
  );

  return {data, error: null, isLoading: false};
};

export default useGetFlowsMeta;
