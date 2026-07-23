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
import {describe, it, expect, vi} from 'vitest';
import {NotificationType} from '../../models/notification';
import type {StepData} from '../../models/steps';
import {computeValidationNotifications} from '../computeValidationNotifications';
import {GRAPH_VALIDATION_RULES, VALIDATION_RULES} from '../validation-rules';

// Simple mock t function that returns the key
const t = vi.fn((key: string) => key) as unknown as import('i18next').TFunction;

/**
 * Helper to create a mock React Flow node with step data.
 */
function createNode(overrides: Partial<Node<StepData>> & {id: string}): Node {
  return {
    position: {x: 0, y: 0},
    data: {},
    ...overrides,
  } as Node;
}

/**
 * Helper to create a mock element within a step's components.
 */
function createElementNode(
  nodeId: string,
  elements: Record<string, unknown>[],
  nodeOverrides: Record<string, unknown> = {},
): Node {
  return createNode({
    id: nodeId,
    data: {components: elements} as unknown as StepData,
    ...nodeOverrides,
  });
}

describe('computeValidationNotifications', () => {
  describe('Required field validation', () => {
    it('should produce notification when a required field is missing', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'input-1', type: 'TEXT_INPUT', category: 'FIELD', config: {field: {}, styles: {}}},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(true);
      const notification = result.get('input-1_REQUIRED_FIELD_ERROR')!;
      expect(notification.getType()).toBe(NotificationType.ERROR);
      expect(notification.hasResourceFieldNotification('input-1_label')).toBe(true);
      expect(notification.hasResourceFieldNotification('input-1_ref')).toBe(true);
    });

    it('should not produce notification when required field is present in config', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'input-1',
            type: 'TEXT_INPUT',
            category: 'FIELD',
            config: {field: {}, styles: {}, label: 'Test Label'},
            ref: 'username',
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(false);
    });

    it('should not produce notification when required field is present on resource directly', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'input-1',
            type: 'TEXT_INPUT',
            category: 'FIELD',
            config: {field: {}, styles: {}},
            label: 'My Label',
            ref: 'field1',
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(false);
    });

    it('should produce notification only for missing fields when some are present', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'input-1', type: 'TEXT_INPUT', category: 'FIELD', config: {field: {}, styles: {}}, label: 'Has Label'},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(true);
      const notification = result.get('input-1_REQUIRED_FIELD_ERROR')!;
      // label is present, so only ref should be missing
      expect(notification.hasResourceFieldNotification('input-1_label')).toBe(false);
      expect(notification.hasResourceFieldNotification('input-1_ref')).toBe(true);
    });
  });

  describe('Nested property validation', () => {
    it('should validate nested properties via dotted path', () => {
      const nodes = [
        createNode({
          id: 'exec-1',
          data: {
            action: {executor: {name: 'GoogleOIDCAuthExecutor'}},
            properties: {idpId: 'google-idp-123'},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('exec-1_REQUIRED_FIELD_ERROR')).toBe(false);
    });

    it('should produce notification for missing nested property', () => {
      const nodes = [
        createNode({
          id: 'exec-1',
          data: {
            action: {executor: {name: 'GoogleOIDCAuthExecutor'}},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('exec-1_REQUIRED_FIELD_ERROR')).toBe(true);
      expect(
        result.get('exec-1_REQUIRED_FIELD_ERROR')!.hasResourceFieldNotification('exec-1_data.properties.idpId'),
      ).toBe(true);
    });

    it('should treat {{IDP_NAME}} placeholder as missing value', () => {
      const nodes = [
        createNode({
          id: 'exec-1',
          data: {
            action: {executor: {name: 'GoogleOIDCAuthExecutor'}},
            properties: {idpId: '{{IDP_NAME}}'},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('exec-1_REQUIRED_FIELD_ERROR')).toBe(true);
    });

    it('should treat {{IDP_ID}} placeholder as missing value', () => {
      const nodes = [
        createNode({
          id: 'exec-1',
          data: {
            action: {executor: {name: 'GithubOAuthExecutor'}},
            properties: {idpId: '{{IDP_ID}}'},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('exec-1_REQUIRED_FIELD_ERROR')).toBe(true);
    });

    it('should treat {{SENDER_ID}} placeholder as missing value', () => {
      const nodes = [
        createNode({
          id: 'sms-1',
          data: {
            action: {executor: {name: 'SMSExecutor'}},
            properties: {senderId: '{{SENDER_ID}}'},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('sms-1_REQUIRED_FIELD_ERROR')).toBe(true);
    });
  });

  describe('Multiple element types', () => {
    it('should validate different element types independently', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'btn-1',
            type: 'ACTION',
            category: 'ACTION',
            config: {field: {}, styles: {}},
            label: 'Submit',
            variant: 'PRIMARY',
          },
          {id: 'input-1', type: 'TEXT_INPUT', category: 'FIELD', config: {field: {}, styles: {}}},
          {
            id: 'divider-1',
            type: 'DIVIDER',
            category: 'DISPLAY',
            config: {field: {}, styles: {}},
            variant: 'HORIZONTAL',
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      // Button and divider have all fields — no error
      expect(result.has('btn-1_REQUIRED_FIELD_ERROR')).toBe(false);
      expect(result.has('divider-1_REQUIRED_FIELD_ERROR')).toBe(false);
      // Input is missing label and ref — error
      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(true);
    });

    it('should validate button with missing variant', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'btn-1', type: 'ACTION', category: 'ACTION', config: {field: {}, styles: {}}, label: 'Click me'},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('btn-1_REQUIRED_FIELD_ERROR')).toBe(true);
      expect(result.get('btn-1_REQUIRED_FIELD_ERROR')!.hasResourceFieldNotification('btn-1_variant')).toBe(true);
      expect(result.get('btn-1_REQUIRED_FIELD_ERROR')!.hasResourceFieldNotification('btn-1_label')).toBe(false);
    });

    it('should validate image with missing src', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'img-1', type: 'IMAGE', category: 'DISPLAY', config: {field: {}, styles: {}}},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('img-1_REQUIRED_FIELD_ERROR')).toBe(true);
      expect(result.get('img-1_REQUIRED_FIELD_ERROR')!.hasResourceFieldNotification('img-1_src')).toBe(true);
    });
  });

  describe('Form structural validation', () => {
    it('should produce notification when form has input fields but no submit button', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'form-1',
            type: 'BLOCK',
            category: 'BLOCK',
            config: {field: {}, styles: {}},
            components: [
              {
                id: 'input-1',
                type: 'TEXT_INPUT',
                category: 'FIELD',
                config: {field: {}, styles: {}},
                label: 'Name',
                ref: 'name',
              },
            ],
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('form-1_FORM_NO_SUBMIT_BUTTON')).toBe(true);
      expect(result.get('form-1_FORM_NO_SUBMIT_BUTTON')!.getType()).toBe(NotificationType.ERROR);
    });

    it('should not produce notification when form has input fields and a submit button', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'form-1',
            type: 'BLOCK',
            category: 'BLOCK',
            config: {field: {}, styles: {}},
            components: [
              {
                id: 'input-1',
                type: 'TEXT_INPUT',
                category: 'FIELD',
                config: {field: {}, styles: {}},
                label: 'Name',
                ref: 'name',
              },
              {
                id: 'btn-1',
                type: 'ACTION',
                category: 'ACTION',
                config: {field: {}, styles: {}},
                label: 'Submit',
                variant: 'PRIMARY',
                eventType: 'SUBMIT',
              },
            ],
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('form-1_FORM_NO_SUBMIT_BUTTON')).toBe(false);
    });

    it('should not produce notification when form has no input fields', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'form-1',
            type: 'BLOCK',
            category: 'BLOCK',
            config: {field: {}, styles: {}},
            components: [
              {
                id: 'text-1',
                type: 'TEXT',
                category: 'DISPLAY',
                config: {field: {}, styles: {}},
                label: 'Title',
                variant: 'HEADING_1',
              },
            ],
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('form-1_FORM_NO_SUBMIT_BUTTON')).toBe(false);
    });

    it('should treat dynamic input placeholder as an input-like field for submit validation', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'form-1',
            type: 'BLOCK',
            category: 'BLOCK',
            components: [
              {
                id: 'dynamic-inputs',
                type: 'DYNAMIC_INPUT_PLACEHOLDER',
                category: 'DISPLAY',
              },
            ],
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('form-1_FORM_NO_SUBMIT_BUTTON')).toBe(true);
    });
  });

  describe('Execution validation', () => {
    it('should validate SMS executor with missing senderId', () => {
      const nodes = [
        createNode({
          id: 'sms-step-1',
          data: {
            action: {executor: {name: 'SMSExecutor'}},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('sms-step-1_REQUIRED_FIELD_ERROR')).toBe(true);
      expect(
        result
          .get('sms-step-1_REQUIRED_FIELD_ERROR')!
          .hasResourceFieldNotification('sms-step-1_data.properties.senderId'),
      ).toBe(true);
    });

    it('should not produce notification for executor with all fields present', () => {
      const nodes = [
        createNode({
          id: 'sms-step-1',
          data: {
            action: {executor: {name: 'SMSExecutor'}},
            properties: {senderId: 'my-sender'},
          } as unknown as StepData,
        }),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.has('sms-step-1_REQUIRED_FIELD_ERROR')).toBe(false);
    });
  });

  describe('Nested element validation', () => {
    it('should validate elements nested inside form components', () => {
      const nodes = [
        createElementNode('step-1', [
          {
            id: 'form-1',
            type: 'BLOCK',
            category: 'BLOCK',
            config: {field: {}, styles: {}},
            components: [{id: 'nested-input', type: 'TEXT_INPUT', category: 'FIELD', config: {field: {}, styles: {}}}],
          },
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      // The nested input should be validated
      expect(result.has('nested-input_REQUIRED_FIELD_ERROR')).toBe(true);
    });
  });

  describe('Edge cases', () => {
    it('should handle empty nodes array', () => {
      const result = computeValidationNotifications([], VALIDATION_RULES, t);

      expect(result.size).toBe(0);
    });

    it('should handle node with no components', () => {
      const nodes = [createNode({id: 'step-1', data: {} as StepData})];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.size).toBe(0);
    });

    it('should handle node with empty components array', () => {
      const nodes = [createElementNode('step-1', [])];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.size).toBe(0);
    });

    it('should handle element with undefined config', () => {
      const nodes = [
        createElementNode('step-1', [{id: 'input-1', type: 'TEXT_INPUT', category: 'FIELD', config: undefined}]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      // Should still produce notification for missing fields without crashing
      expect(result.has('input-1_REQUIRED_FIELD_ERROR')).toBe(true);
    });

    it('should not produce notifications for unrecognized element types', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'custom-1', type: 'CUSTOM_UNKNOWN', category: 'DISPLAY', config: {field: {}, styles: {}}},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.size).toBe(0);
    });

    it('should automatically remove notifications when element is removed from nodes', () => {
      // First computation with element
      const nodesWithElement = [
        createElementNode('step-1', [
          {id: 'input-1', type: 'TEXT_INPUT', category: 'FIELD', config: {field: {}, styles: {}}},
        ]),
      ];

      const result1 = computeValidationNotifications(nodesWithElement, VALIDATION_RULES, t);
      expect(result1.has('input-1_REQUIRED_FIELD_ERROR')).toBe(true);

      // Second computation without element — notification gone
      const nodesWithoutElement = [createElementNode('step-1', [])];

      const result2 = computeValidationNotifications(nodesWithoutElement, VALIDATION_RULES, t);
      expect(result2.has('input-1_REQUIRED_FIELD_ERROR')).toBe(false);
    });

    it('should include resource reference on notification for panel navigation', () => {
      const nodes = [
        createElementNode('step-1', [
          {id: 'btn-1', type: 'ACTION', category: 'ACTION', config: {field: {}, styles: {}}},
        ]),
      ];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      const notification = result.get('btn-1_REQUIRED_FIELD_ERROR')!;
      expect(notification.hasResource('btn-1')).toBe(true);
    });
  });

  describe('SSO pairing rule', () => {
    function createSsoCheckNode(id: string, checkpointRef?: string): Node {
      return createNode({
        id,
        type: 'TASK_EXECUTION',
        data: {
          action: {type: 'EXECUTOR', executor: {name: 'SSOCheckExecutor'}},
          ...(checkpointRef !== undefined ? {properties: {checkpointRef}} : {}),
        } as unknown as StepData,
      });
    }

    function createSessionNode(id: string): Node {
      return createNode({
        id,
        type: 'TASK_EXECUTION',
        data: {action: {type: 'EXECUTOR', executor: {name: 'SessionExecutor'}}} as unknown as StepData,
      });
    }

    it('should not run graph rules by default (flow-type-specific opt-in)', () => {
      const nodes = [createSsoCheckNode('sso_check_1'), createSessionNode('session_1')];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t);

      expect(result.size).toBe(0);
    });

    it('should not report anything for a healthy pair', () => {
      const nodes = [createSsoCheckNode('sso_check_1', 'session_1'), createSessionNode('session_1')];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t, GRAPH_VALIDATION_RULES);

      expect(result.size).toBe(0);
    });

    it('should report an SSO check with a missing checkpointRef', () => {
      const nodes = [createSsoCheckNode('sso_check_1'), createSessionNode('session_1')];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t, GRAPH_VALIDATION_RULES);

      expect(result.has('sso_check_1_SSO_MISSING_CHECKPOINT_REF')).toBe(true);
      expect(result.get('sso_check_1_SSO_MISSING_CHECKPOINT_REF')!.getType()).toBe(NotificationType.ERROR);
      // The session is unreferenced as a consequence.
      expect(result.has('session_1_SSO_ORPHAN_SESSION')).toBe(true);
    });

    it('should report an SSO check whose checkpointRef points to a deleted session', () => {
      const nodes = [createSsoCheckNode('sso_check_1', 'session_gone')];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t, GRAPH_VALIDATION_RULES);

      expect(result.has('sso_check_1_SSO_INVALID_CHECKPOINT_REF')).toBe(true);
    });

    it('should report a session node not referenced by any SSO check', () => {
      const nodes = [createSessionNode('session_1')];

      const result = computeValidationNotifications(nodes, VALIDATION_RULES, t, GRAPH_VALIDATION_RULES);

      expect(result.has('session_1_SSO_ORPHAN_SESSION')).toBe(true);
      const notification = result.get('session_1_SSO_ORPHAN_SESSION')!;
      expect(notification.getType()).toBe(NotificationType.ERROR);
      expect(notification.hasResource('session_1')).toBe(true);
    });
  });
});
