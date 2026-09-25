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
 * - Sets senderId in data.properties for the SMS and Email executors
 *
 * Only auto-assigns when there's exactly one connection configured, except for the Email
 * executor, which always takes the first provider because an email step is unusable without one.
 * Otherwise, when there are multiple connections, the user should select one from the resource
 * panel.
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

      // The Email executor takes the first provider whatever the count: an email step is unusable
      // without one, and the user can switch it in the resource panel. Every other executor only
      // auto-assigns when a single configured connection makes the choice unambiguous.
      const isEmailExecutor: boolean = executorName === ExecutionTypes.EmailExecutor;
      if (!firstConnection || (!isEmailExecutor && connections.length !== 1)) {
        return;
      }

      // Handle the sender-backed executors (SMS, Email) - both use senderId
      if (executorName === ExecutionTypes.SMSExecutor || executorName === ExecutionTypes.EmailExecutor) {
        if (properties?.senderId === SENDER_ID_PLACEHOLDER || properties?.senderId === '' || !properties?.senderId) {
          // Initialize properties if needed
          step.data.properties ??= {};
          (step.data.properties as Record<string, string>).senderId = firstConnection;
        }
        return;
      }

      // Handle IDP-based executors (Google, GitHub, etc.) - uses idpId
      if (properties?.idpId === IDP_ID_PLACEHOLDER || properties?.idpId === '' || !properties?.idpId) {
        // Initialize properties if needed
        step.data.properties ??= {};
        (step.data.properties as Record<string, string>).idpId = firstConnection;
      }
    }
  });
};

export default autoAssignConnections;
