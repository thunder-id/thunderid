/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import type {ExecutorConnectionInterface} from '../../models/metadata';
import {ExecutionTypes, StepTypes} from '../../models/steps';
import autoAssignConnections from '../autoAssignConnections';

describe('autoAssignConnections', () => {
  const createNode = (
    id: string,
    type: string,
    executorName?: string,
    properties?: {idpId?: string; senderId?: string},
  ): Node => ({
    id,
    type,
    position: {x: 0, y: 0},
    data: {
      action: executorName ? {executor: {name: executorName}} : undefined,
      properties,
    },
  });

  describe('IDP-based Executors', () => {
    it('should auto-assign idpId when there is exactly one connection', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('google-idp-1');
    });

    it('should auto-assign idpId when properties.idpId is placeholder', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GithubFederation, {idpId: '{{IDP_ID}}'})];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GithubFederation, connections: ['github-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('github-idp-1');
    });

    it('should auto-assign idpId when properties.idpId is empty string', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation, {idpId: ''})];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('google-idp-1');
    });

    it('should not overwrite existing idpId', () => {
      const nodes = [
        createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation, {idpId: 'existing-idp'}),
      ];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['new-idp']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('existing-idp');
    });

    it('should not auto-assign when there are multiple connections', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1', 'google-idp-2']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should not auto-assign when there are no connections', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: []},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });
  });

  describe('SMS OTP Executor', () => {
    it('should auto-assign senderId when there is exactly one connection', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.SMSExecutor)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.SMSExecutor, connections: ['sms-sender-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {senderId: string}).senderId).toBe('sms-sender-1');
    });

    it('should auto-assign senderId when properties.senderId is placeholder', () => {
      const nodes = [
        createNode('node-1', StepTypes.Execution, ExecutionTypes.SMSExecutor, {senderId: '{{SENDER_ID}}'}),
      ];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.SMSExecutor, connections: ['sms-sender-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {senderId: string}).senderId).toBe('sms-sender-1');
    });

    it('should auto-assign senderId when properties.senderId is empty string', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.SMSExecutor, {senderId: ''})];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.SMSExecutor, connections: ['sms-sender-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {senderId: string}).senderId).toBe('sms-sender-1');
    });

    it('should not overwrite existing senderId', () => {
      const nodes = [
        createNode('node-1', StepTypes.Execution, ExecutionTypes.SMSExecutor, {senderId: 'existing-sender'}),
      ];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.SMSExecutor, connections: ['new-sender']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {senderId: string}).senderId).toBe('existing-sender');
    });

    it('should not auto-assign when there are multiple SMS senders', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.SMSExecutor)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.SMSExecutor, connections: ['sender-1', 'sender-2']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });
  });

  describe('Non-Execution Nodes', () => {
    it('should skip non-execution step nodes', () => {
      const nodes = [createNode('node-1', StepTypes.View, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should skip END type nodes', () => {
      const nodes = [createNode('node-1', StepTypes.End, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should skip RULE type nodes', () => {
      const nodes = [createNode('node-1', StepTypes.Rule, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });
  });

  describe('Missing Executor Name', () => {
    it('should skip nodes without executor name', () => {
      const nodes = [createNode('node-1', StepTypes.Execution)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should skip nodes with missing action', () => {
      const node: Node = {
        id: 'node-1',
        type: StepTypes.Execution,
        position: {x: 0, y: 0},
        data: {},
      };
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections([node], connections);

      expect(node.data.properties).toBeUndefined();
    });
  });

  describe('Multiple Nodes', () => {
    it('should process multiple execution nodes', () => {
      const nodes = [
        createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation),
        createNode('node-2', StepTypes.Execution, ExecutionTypes.GithubFederation),
        createNode('node-3', StepTypes.Execution, ExecutionTypes.SMSExecutor),
      ];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
        {executorName: ExecutionTypes.GithubFederation, connections: ['github-idp-1']},
        {executorName: ExecutionTypes.SMSExecutor, connections: ['sms-sender-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('google-idp-1');
      expect((nodes[1].data.properties as {idpId: string}).idpId).toBe('github-idp-1');
      expect((nodes[2].data.properties as {senderId: string}).senderId).toBe('sms-sender-1');
    });

    it('should handle mixed node types', () => {
      const nodes = [
        createNode('node-1', StepTypes.View),
        createNode('node-2', StepTypes.Execution, ExecutionTypes.GoogleFederation),
        createNode('node-3', StepTypes.End),
      ];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
      expect((nodes[1].data.properties as {idpId: string}).idpId).toBe('google-idp-1');
      expect(nodes[2].data.properties).toBeUndefined();
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty nodes array', () => {
      const nodes: Node[] = [];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      expect(() => autoAssignConnections(nodes, connections)).not.toThrow();
    });

    it('should handle empty connections array', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should handle executor without matching connections', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GithubFederation, connections: ['github-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect(nodes[0].data.properties).toBeUndefined();
    });

    it('should mutate the original nodes array', () => {
      const nodes = [createNode('node-1', StepTypes.Execution, ExecutionTypes.GoogleFederation)];
      const connections: ExecutorConnectionInterface[] = [
        {executorName: ExecutionTypes.GoogleFederation, connections: ['google-idp-1']},
      ];

      autoAssignConnections(nodes, connections);

      expect((nodes[0].data.properties as {idpId: string}).idpId).toBe('google-idp-1');
    });
  });
});
