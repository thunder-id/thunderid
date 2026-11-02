// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import merge from 'lodash-es/merge';
import type {Resources} from '../models/resources';
import type {Step, StepData} from '../models/steps';

/**
 * Type-safe wrapper for lodash merge function.
 *
 * @param sources - Source objects to merge from.
 * @returns The merged object.
 */
const safeMerge = <T>(...sources: Partial<T>[]): T => (merge as (...args: Partial<T>[]) => T)(...sources);

const resolveStepMetadata = (resources: Resources, steps: Step[]): Step[] => {
  const updateStepResourceType = (step: Step): Step => {
    let updatedStep: Step = {...step};

    const stepWithMeta = resources?.steps?.find((s) => s.type === step.type);

    if (stepWithMeta) {
      updatedStep = safeMerge<Step>({}, stepWithMeta, updatedStep);
    }

    // For EXECUTION type steps, also check executors for metadata based on executor name
    const stepData = step.data as StepData | undefined;
    const executorName = stepData?.action?.executor?.name;
    const executorMode = (stepData?.action?.executor as {mode?: string} | undefined)?.mode;

    if (executorName && resources?.executors) {
      // For executors with modes (like OTPExecutor), match on both name and mode
      const executorWithMeta = resources.executors.find((executor) => {
        const executorData = executor.data as StepData | undefined;
        const metaExecutorName = executorData?.action?.executor?.name;
        const metaExecutorMode = (executorData?.action?.executor as {mode?: string} | undefined)?.mode;

        // Match by name first
        if (metaExecutorName !== executorName) {
          return false;
        }

        // If the step has a mode, try to match it; otherwise, use the first matching executor
        if (executorMode && metaExecutorMode) {
          return metaExecutorMode === executorMode;
        }

        return true;
      });

      if (executorWithMeta) {
        const defaultProps = (executorWithMeta.data as StepData & {properties?: Record<string, unknown>})?.properties;
        const existingProps = (updatedStep.data as StepData & {properties?: Record<string, unknown>})?.properties;

        if (defaultProps && existingProps) {
          const coercedProps = Object.fromEntries(
            Object.entries(existingProps).map(([key, value]) => {
              const defaultValue = defaultProps[key];

              if (typeof defaultValue === 'number' && typeof value === 'string') {
                const trimmedValue = value.trim();

                if (trimmedValue === '') {
                  return [key, defaultValue];
                }

                const numericValue = Number(trimmedValue);

                return [key, Number.isFinite(numericValue) ? numericValue : defaultValue];
              }

              if (typeof defaultValue === 'boolean' && typeof value === 'string') {
                const normalizedValue = value.trim().toLowerCase();

                if (normalizedValue === 'true') {
                  return [key, true];
                }

                if (normalizedValue === 'false') {
                  return [key, false];
                }

                return [key, defaultValue];
              }

              return [key, value];
            }),
          );

          updatedStep = {
            ...updatedStep,
            data: {
              ...updatedStep.data,
              properties: coercedProps,
            },
          };
        }

        // Merge executor display metadata into the step (at root level and in data for React Flow access)
        updatedStep = {
          ...updatedStep,
          display: executorWithMeta.display,
          data: {
            ...updatedStep.data,
            display: executorWithMeta.display,
          },
        };
      }
    }

    return updatedStep;
  };

  return steps?.map(updateStepResourceType);
};

export default resolveStepMetadata;
