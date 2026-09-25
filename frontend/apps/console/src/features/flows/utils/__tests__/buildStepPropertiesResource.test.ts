// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, expect, it} from 'vitest';
import type {Resource} from '../../models/resources';
import type {Step} from '../../models/steps';
import buildStepPropertiesResource from '../buildStepPropertiesResource';

describe('buildStepPropertiesResource', () => {
  it('should build an EXECUTION resource mirroring the execution node configure action', () => {
    const node: Node = {
      id: 'task_execution_abcd',
      type: 'TASK_EXECUTION',
      position: {x: 0, y: 0},
      data: {
        action: {executor: {name: 'OTPExecutor'}},
        display: {label: 'Email OTP', image: 'MailIcon', description: 'Sends an OTP'},
      },
    };

    const resource = buildStepPropertiesResource(node, []) as Step;

    expect(resource).not.toBeNull();
    expect(resource.id).toBe('task_execution_abcd');
    expect(resource.type).toBe('EXECUTION');
    expect(resource.category).toBe('WORKFLOW');
    expect(resource.resourceType).toBe('STEP');
    expect(resource.data).toBe(node.data);
    expect(resource.display?.label).toBe('Email OTP');
    expect(resource.display?.image).toBe('MailIcon');
    expect(resource.display?.showOnResourcePanel).toBe(false);
  });

  it('should fall back to the executor name when the execution node has no display label', () => {
    const node: Node = {
      id: 'task_execution_abcd',
      type: 'TASK_EXECUTION',
      position: {x: 0, y: 0},
      data: {action: {executor: {name: 'OTPExecutor'}}},
    };

    const resource = buildStepPropertiesResource(node, []) as Step;

    expect(resource.display?.label).toBe('OTPExecutor');
  });

  it('should build a RULE resource by spreading the node data', () => {
    const node: Node = {
      id: 'rule_abcd',
      type: 'RULE',
      position: {x: 0, y: 0},
      data: {resourceType: 'STEP', category: 'DECISION'},
    };

    const resource = buildStepPropertiesResource(node, []);

    expect(resource).toMatchObject({id: 'rule_abcd', resourceType: 'STEP', category: 'DECISION'});
  });

  it('should build a CALL resource from its palette entry', () => {
    const paletteEntry = {
      type: 'CALL',
      category: 'WORKFLOW',
      resourceType: 'STEP',
      display: {label: 'Flow', image: 'WorkflowIcon', showOnResourcePanel: true},
    } as unknown as Resource;
    const node: Node = {
      id: 'call_abcd',
      type: 'CALL',
      position: {x: 0, y: 0},
      data: {flow: {ref: 'other-flow'}},
    };

    const resource = buildStepPropertiesResource(node, [paletteEntry]) as Step;

    expect(resource.id).toBe('call_abcd');
    expect(resource.type).toBe('CALL');
    expect(resource.category).toBe('WORKFLOW');
    expect(resource.data).toEqual({flow: {ref: 'other-flow'}});
    expect(resource.display?.label).toBe('Flow');
    expect(resource.display?.image).toBe('WorkflowIcon');
    expect(resource.display?.showOnResourcePanel).toBe(false);
  });

  it('should return null for node types without a properties panel', () => {
    const positions = {position: {x: 0, y: 0}, data: {}};

    expect(buildStepPropertiesResource({id: 'view_1', type: 'VIEW', ...positions}, [])).toBeNull();
    expect(buildStepPropertiesResource({id: 'start', type: 'START', ...positions}, [])).toBeNull();
    expect(buildStepPropertiesResource({id: 'end', type: 'END', ...positions}, [])).toBeNull();
  });
});
