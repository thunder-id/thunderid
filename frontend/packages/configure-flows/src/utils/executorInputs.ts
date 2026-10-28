// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Resource} from '../models/resources';
import type {FlowNodeInput} from '../models/responses';
import type {StepData} from '../models/steps';

/**
 * Returns the inputs configured on a step's executor, or none when the resource has no executor.
 *
 * @param resource - The step resource.
 * @returns The executor inputs.
 */
export const getExecutorInputs = (resource: Resource): FlowNodeInput[] => {
  const inputs = ((resource as {data?: StepData}).data?.action?.executor as {inputs?: unknown} | undefined)?.inputs;

  return Array.isArray(inputs) ? (inputs as FlowNodeInput[]) : [];
};

/**
 * Reports whether an input's default and constant settings cannot be applied. A constant input needs a
 * default value to supply.
 *
 * @param input - The executor input.
 * @returns True when the input is not configured correctly.
 */
export const hasInvalidInputConfig = (input: FlowNodeInput): boolean => input.constant === true && !input.default;

/**
 * Input type of a boolean field. Its default value is picked from true and false.
 */
export const BOOLEAN_INPUT_TYPE = 'BOOLEAN_INPUT';

const TRUE_SPELLINGS: readonly string[] = ['1', 't', 'T', 'TRUE', 'true', 'True'];
const FALSE_SPELLINGS: readonly string[] = ['0', 'f', 'F', 'FALSE', 'false', 'False'];

/**
 * Reads a boolean default as `true` or `false`, accepting the spellings the server accepts.
 *
 * @param value - The stored default value.
 * @returns `true` or `false`, or an empty string when the value is missing or not a boolean.
 */
export const toBooleanDefault = (value: string | undefined): 'true' | 'false' | '' => {
  if (TRUE_SPELLINGS.includes(value ?? '')) {
    return 'true';
  }

  return FALSE_SPELLINGS.includes(value ?? '') ? 'false' : '';
};
