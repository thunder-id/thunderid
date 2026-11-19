// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  RuntimeEnvironmentsContext,
  type EnvironmentValue,
  type EnvironmentValueField,
  type RuntimeEnvironment,
  type RuntimeEnvironmentsContextType,
  useToast,
} from '@thunderid/contexts';
import {useEffect, useMemo, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useDeploySource from '../api/source';
import {usePipeline} from '../api/usePipeline';
import {gatewayColor} from '../constants/gateway-colors';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {StoredValue, ValueReference} from '../models/deploy';
import type {PipelineStage} from '../models/pipeline';

/**
 * Tells the pages below which environments the configuration is deployed to, as the pipeline orders them,
 * and reads what each holds through the source in effect: the control plane's gateways, or the environments
 * a console provides its own source for.
 *
 * A stage whose target names no base URL has no endpoints to show, so it is left out. Where the pipeline
 * cannot be read there are no environments to tell of, and the pages below show the default one's values,
 * so a toast says so.
 */
export default function RuntimeEnvironmentsProvider({children}: {children: ReactNode}): JSX.Element {
  const source = useDeploySource();
  const {t} = useTranslation();
  const {showToast} = useToast();
  const {data: pipeline, isError} = usePipeline();
  const deployPath = useDeployRoutes().deploy.pipeline?.();

  useEffect(() => {
    if (isError) {
      showToast(
        t(
          'deploy:environments.loadFailed',
          'The environments could not be loaded. The values shown are for the default environment.',
        ),
        'error',
      );
    }
  }, [isError, showToast, t]);

  const value: RuntimeEnvironmentsContextType = useMemo(
    () => ({
      deployPath,
      // Each keeps the color of its place in the pipeline, as the rest of the console marks it.
      environments: (pipeline?.stages ?? []).flatMap((stage: PipelineStage, index: number): RuntimeEnvironment[] => {
        const runtimeUrl = stage.gateway.baseUrl.replace(/\/+$/, '');
        return runtimeUrl ? [{id: stage.gateway.id, name: stage.name, runtimeUrl, color: gatewayColor(index)}] : [];
      }),
      // A field's value is held under the name the running version refers to it by. An environment whose
      // running version refers to none for the field does not run the resource.
      getValue: async (
        environmentId: string,
        {resourceType, resourceId, field}: EnvironmentValueField,
      ): Promise<EnvironmentValue> => {
        const reference = (await source.runningReferences(environmentId)).find(
          (candidate: ValueReference) =>
            candidate.resourceType === resourceType && candidate.resourceId === resourceId && candidate.field === field,
        );
        if (!reference) return {status: 'absent'};
        // Only the one name is looked up, not everything the environment holds.
        const page = await source.listValues(environmentId, reference.kind, {names: [reference.name], limit: 1});
        const stored = page.values.find((candidate: StoredValue) => candidate.name === reference.name)?.value;
        return stored ? {status: 'set', value: stored} : {status: 'unset'};
      },
    }),
    [pipeline, source, deployPath],
  );

  return <RuntimeEnvironmentsContext.Provider value={value}>{children}</RuntimeEnvironmentsContext.Provider>;
}
