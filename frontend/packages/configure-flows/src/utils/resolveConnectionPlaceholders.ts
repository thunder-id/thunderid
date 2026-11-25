// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EXECUTOR_TO_IDP_TYPE_MAP} from '../components/resource-property-panel/extended-properties/execution-properties/constants';
import type {FlowNode} from '../models/responses';
import {ExecutionTypes} from '../models/steps';

const IDP_ID_PLACEHOLDER = '{{IDP_ID}}';
const SENDER_ID_PLACEHOLDER = '{{SENDER_ID}}';

/**
 * The connections configured in the deployment. A list is undefined when it could not be read, and
 * the executors that use it are then left as they are.
 */
export interface AvailableConnections {
  identityProviders?: {id: string; type: string}[];
  smsProviders?: {id: string}[];
  emailProviders?: {id: string}[];
}

const isUnresolved = (value: unknown, placeholder: string): boolean =>
  value === undefined || value === '' || value === placeholder;

/**
 * Picks the connection for an executor whose connection property is unset or still a template placeholder.
 *
 * The Email executor takes the first provider, since an email step is unusable without one. The SMS and
 * federated executors take a connection only when exactly one of the matching kind exists.
 *
 * @param executorName - The executor's name.
 * @param properties - The node's properties.
 * @param connections - The configured connections.
 * @returns The property to set, such as `{idpId: '<id>'}`, or null when there is nothing to assign.
 */
export const resolveConnectionProperty = (
  executorName: string,
  properties: Record<string, unknown> | undefined,
  connections: AvailableConnections,
): Record<string, string> | null => {
  const {emailProviders, smsProviders, identityProviders} = connections;

  if (executorName === ExecutionTypes.EmailExecutor) {
    if (!isUnresolved(properties?.['senderId'], SENDER_ID_PLACEHOLDER) || !emailProviders?.length) return null;
    return {senderId: emailProviders[0].id};
  }

  if (executorName === ExecutionTypes.SMSExecutor) {
    if (!isUnresolved(properties?.['senderId'], SENDER_ID_PLACEHOLDER) || smsProviders?.length !== 1) return null;
    return {senderId: smsProviders[0].id};
  }

  const idpType = EXECUTOR_TO_IDP_TYPE_MAP[executorName];
  if (!idpType || !identityProviders || !isUnresolved(properties?.['idpId'], IDP_ID_PLACEHOLDER)) return null;

  const matching = identityProviders.filter((idp) => idp.type === idpType);
  return matching.length === 1 ? {idpId: matching[0].id} : null;
};

/**
 * Fills in the connections of a flow definition's executor nodes, such as a template's, before it is saved.
 *
 * @param nodes - The flow definition's nodes.
 * @param connections - The configured connections.
 * @returns The nodes, with each resolvable connection property set.
 */
const resolveConnectionPlaceholders = (nodes: FlowNode[], connections: AvailableConnections): FlowNode[] =>
  nodes.map((node: FlowNode) => {
    const executorName = node.executor?.name;
    if (!executorName) return node;

    const resolved = resolveConnectionProperty(executorName, node.properties, connections);
    return resolved ? {...node, properties: {...node.properties, ...resolved}} : node;
  });

export default resolveConnectionPlaceholders;
