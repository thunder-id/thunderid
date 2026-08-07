// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {ExecutionTypes, type StepData} from '@/features/flows/models/steps';

/**
 * Digit count previewed when no upstream Generate OTP node resolves a usable length.
 * Matches the server default.
 */
export const DEFAULT_OTP_LENGTH = 6;

/**
 * Bounds the backend accepts for the `otpLength` executor property. A value outside this range is
 * ignored at runtime in favour of the server default, so the preview follows suit.
 */
const MIN_OTP_LENGTH = 4;
const MAX_OTP_LENGTH = 10;

const OTP_GENERATE_MODE = 'generate';

/**
 * Checks whether a node is an OTP executor running in generate mode.
 *
 * @param node - The canvas node to inspect.
 * @returns Whether the node mints an OTP.
 */
const isOtpGenerateNode = (node: Node): boolean => {
  const executor = (node.data as StepData | undefined)?.action?.executor;

  return executor?.name === ExecutionTypes.OTPExecutor && executor?.mode === OTP_GENERATE_MODE;
};

/**
 * Reads a usable `otpLength` from a node, or undefined when it is unset or out of range.
 *
 * @param node - The Generate OTP node.
 * @returns The configured length, or undefined.
 */
const readOtpLength = (node: Node): number | undefined => {
  const length = Number((node.data as StepData | undefined)?.properties?.['otpLength']);

  if (!Number.isInteger(length) || length < MIN_OTP_LENGTH || length > MAX_OTP_LENGTH) {
    return undefined;
  }

  return length;
};

/**
 * Resolves the number of OTP boxes to preview for a step by walking the flow backwards to the
 * nearest Generate OTP node. This mirrors runtime behaviour, where the code a prompt collects is
 * the one minted by the generate step that precedes it, so a flow with two OTP legs previews each
 * leg with its own length.
 *
 * @param stepId - The step the OTP element resides on.
 * @param nodes - All canvas nodes.
 * @param edges - All canvas edges.
 * @returns The configured OTP length, or {@link DEFAULT_OTP_LENGTH} when it cannot be resolved.
 */
const resolveUpstreamOtpLength = (stepId: string, nodes: Node[], edges: Edge[]): number => {
  const nodesById = new Map(nodes.map((node) => [node.id, node]));
  const visited = new Set<string>([stepId]);
  let frontier: string[] = [stepId];

  while (frontier.length > 0) {
    const predecessors: string[] = [];

    edges.forEach((edge) => {
      if (!frontier.includes(edge.target) || visited.has(edge.source)) return;
      visited.add(edge.source);
      predecessors.push(edge.source);
    });

    const generateNode = predecessors
      .map((id) => nodesById.get(id))
      .find((node): node is Node => !!node && isOtpGenerateNode(node));

    if (generateNode) {
      return readOtpLength(generateNode) ?? DEFAULT_OTP_LENGTH;
    }

    frontier = predecessors;
  }

  return DEFAULT_OTP_LENGTH;
};

export default resolveUpstreamOtpLength;
