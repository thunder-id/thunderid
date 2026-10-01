// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import type {ExecutorConnectionInterface} from '../models/metadata';
import {ExecutionTypes, type Step, StepTypes} from '../models/steps';

const IDP_ID_PLACEHOLDER = '{{IDP_ID}}';
const SENDER_ID_PLACEHOLDER = '{{SENDER_ID}}';

/**
 * Automatically assigns connections to nodes based on available connections.
 * - Sets idpId in data.properties for IDP-based executors (Google, GitHub, etc.)
 * - Sets senderId in data.properties for SMS OTP executor
 *
 * Only auto-assigns when there's exactly one connection configured.
 * If there are multiple connections, the user should select one from the resource panel.
 *
 * @param nodes - The array of nodes to process.
 * @param availableConnections - The array of available executor connections.
 */
const autoAssignConnections = (nodes: Node[], availableConnections: ExecutorConnectionInterface[]) => {
  const availableConnectionsMap: Record<string, string[]> = availableConnections.reduce(
    (map: Record<string, string[]>, executorConnections: ExecutorConnectionInterface) => ({
      ...map,
      [executorConnections.executorName]: executorConnections.connections,
    }),
    {} as Record<string, string[]>,
  );

  nodes.forEach((node: Node) => {
    // Only process execution step nodes.
    if (node.type === StepTypes.Execution) {
      const step: Step = node as Step;
      const action = step.data?.action as {executor?: {name?: string}} | undefined;
      const properties = step.data?.properties as {idpId?: string; senderId?: string} | undefined;
      const executorName = action?.executor?.name;

      if (typeof executorName !== 'string') {
        return;
      }

      const connections: string[] = availableConnectionsMap[executorName] ?? [];
      const [firstConnection] = connections;

      // Only auto-assign if there's exactly one connection configured.
      if (connections.length !== 1 || !firstConnection) {
        return;
      }

      // Handle SMS executor - uses senderId
      if (executorName === ExecutionTypes.SMSExecutor) {
        if (properties?.senderId === SENDER_ID_PLACEHOLDER || properties?.senderId === '' || !properties?.senderId) {
          // Initialize properties if needed
          step.data.properties ??= {};
          (step.data.properties as Record<string, string>)['senderId'] = firstConnection;
        }
        return;
      }

      // Handle IDP-based executors (Google, GitHub, etc.) - uses idpId
      if (properties?.idpId === IDP_ID_PLACEHOLDER || properties?.idpId === '' || !properties?.idpId) {
        // Initialize properties if needed
        step.data.properties ??= {};
        (step.data.properties as Record<string, string>)['idpId'] = firstConnection;
      }
    }
  });
};

export default autoAssignConnections;
