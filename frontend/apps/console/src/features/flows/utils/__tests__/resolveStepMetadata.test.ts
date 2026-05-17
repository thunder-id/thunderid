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

import {describe, expect, it} from 'vitest';
import type {Resources} from '../../models/resources';
import type {Step} from '../../models/steps';
import resolveStepMetadata from '../resolveStepMetadata';

const createMockStep = (overrides: Partial<Step> = {}): Step =>
  ({
    id: 'step-1',
    type: 'VIEW',
    category: 'INTERFACE',
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'STEP',
    display: {
      label: 'Step Label',
      image: '',
      showOnResourcePanel: true,
    },
    config: {
      field: {name: '', type: 'TEXT_INPUT'},
      styles: {},
    },
    size: {width: 200, height: 100},
    position: {x: 0, y: 0},
    __generationMeta__: null,
    data: {},
    ...overrides,
  }) as Step;

const createMockResources = (overrides: Partial<Resources> = {}): Resources => ({
  elements: [],
  steps: [],
  widgets: [],
  templates: [],
  executors: [],
  ...overrides,
});

describe('resolveStepMetadata', () => {
  describe('Basic Step Metadata Resolution', () => {
    it('should return steps with merged metadata from resources', () => {
      // Step with minimal display - metadata should fill in gaps
      const steps: Step[] = [
        createMockStep({
          id: 'step-1',
          type: 'VIEW',
          display: {
            label: '', // Empty, should get filled from metadata
            image: '', // Empty, should get filled from metadata
            showOnResourcePanel: false,
          },
        }),
      ];

      const resources = createMockResources({
        steps: [
          createMockStep({
            type: 'VIEW',
            display: {
              label: 'View Step',
              image: '/images/view.svg',
              showOnResourcePanel: true,
              description: 'A view step',
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
      // lodash merge: step values override metadata values, so original values are preserved
      // But metadata's additional properties (description) should be added
      expect(result[0].display.description).toBe('A view step');
    });

    it('should preserve original step data while merging metadata', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'unique-step-id',
          type: 'VIEW',
          position: {x: 100, y: 200},
        }),
      ];

      const resources = createMockResources({
        steps: [
          createMockStep({
            type: 'VIEW',
            display: {
              label: 'View Step',
              image: '/images/view.svg',
              showOnResourcePanel: true,
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].id).toBe('unique-step-id');
      expect(result[0].position).toEqual({x: 100, y: 200});
    });

    it('should handle steps without matching metadata in resources', () => {
      const steps: Step[] = [createMockStep({id: 'step-1', type: 'CUSTOM_TYPE'})];

      const resources = createMockResources({
        steps: [createMockStep({type: 'VIEW'})],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
      expect(result[0].type).toBe('CUSTOM_TYPE');
    });

    it('should handle multiple steps and add metadata properties', () => {
      const steps: Step[] = [
        createMockStep({id: 'step-1', type: 'VIEW'}),
        createMockStep({id: 'step-2', type: 'RULE'}),
        createMockStep({id: 'step-3', type: 'END'}),
      ];

      const resources = createMockResources({
        steps: [
          createMockStep({
            type: 'VIEW',
            display: {label: 'View', image: '', showOnResourcePanel: true, description: 'View step'},
          }),
          createMockStep({
            type: 'RULE',
            display: {label: 'Rule', image: '', showOnResourcePanel: true, description: 'Rule step'},
          }),
          createMockStep({
            type: 'END',
            display: {label: 'End', image: '', showOnResourcePanel: true, description: 'End step'},
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(3);
      // Step original values are preserved, but metadata's additional properties are added
      expect(result[0].display.description).toBe('View step');
      expect(result[1].display.description).toBe('Rule step');
      expect(result[2].display.description).toBe('End step');
    });
  });

  describe('Executor Metadata Resolution', () => {
    it('should resolve executor metadata based on executor name', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'exec-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'GoogleOIDCAuthExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        steps: [createMockStep({type: 'TASK_EXECUTION'})],
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Google Login',
              image: '/images/google.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'GoogleOIDCAuthExecutor'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('Google Login');
      expect(result[0].display.image).toBe('/images/google.svg');
    });

    it('should resolve executor metadata with mode matching', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'sms-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'SMSOTPAuthExecutor', mode: 'SEND'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        steps: [],
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Send SMS OTP',
              image: '/images/sms-send.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'SMSOTPAuthExecutor', mode: 'SEND'},
              },
            },
          }),
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Verify SMS OTP',
              image: '/images/sms-verify.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'SMSOTPAuthExecutor', mode: 'VERIFY'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('Send SMS OTP');
      expect(result[0].display.image).toBe('/images/sms-send.svg');
    });

    it('should fall back to first matching executor when step has no mode', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'sms-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'SMSOTPAuthExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'SMS OTP First',
              image: '/images/sms.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'SMSOTPAuthExecutor', mode: 'SEND'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('SMS OTP First');
    });

    it('should merge executor display into step data as well', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'exec-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'GoogleOIDCAuthExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Google Login',
              image: '/images/google.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'GoogleOIDCAuthExecutor'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].data?.display).toEqual({
        label: 'Google Login',
        image: '/images/google.svg',
        showOnResourcePanel: true,
      });
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty steps array', () => {
      const resources = createMockResources();
      const result = resolveStepMetadata(resources, []);

      expect(result).toEqual([]);
    });

    it('should handle empty resources', () => {
      const steps: Step[] = [createMockStep({id: 'step-1', type: 'VIEW'})];
      const resources = createMockResources();

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
      expect(result[0].type).toBe('VIEW');
    });

    it('should handle undefined steps array', () => {
      const resources = createMockResources();
      const result = resolveStepMetadata(resources, undefined as unknown as Step[]);

      expect(result).toBeUndefined();
    });

    it('should handle null executors in resources', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'step-1',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'TestExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: undefined as unknown as Step[],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
    });

    it('should handle step without data', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'step-1',
          type: 'VIEW',
          data: undefined,
        }),
      ];

      const resources = createMockResources({
        steps: [createMockStep({type: 'VIEW'})],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
    });

    it('should handle step with data but no action', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'step-1',
          type: 'TASK_EXECUTION',
          data: {components: []},
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            data: {
              action: {executor: {name: 'TestExecutor'}},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result).toHaveLength(1);
    });

    it('should coerce string property values to the type defined in executor defaults', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {maxPerPrompt: '3', includeOptional: 'true', assignGroup: 'some-group'},
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            display: {label: 'Provisioning', image: '', showOnResourcePanel: true},
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {maxPerPrompt: 5, includeOptional: false, assignGroup: ''},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const props = (result[0].data as {properties?: Record<string, unknown>})?.properties;

      expect(props?.maxPerPrompt).toBe(3);
      expect(typeof props?.maxPerPrompt).toBe('number');
      expect(props?.includeOptional).toBe(true);
      expect(typeof props?.includeOptional).toBe('boolean');
      expect(props?.assignGroup).toBe('some-group');
    });

    it('should leave property unchanged when string cannot be coerced to the expected type', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {maxPerPrompt: 'not-a-number'},
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            display: {label: 'Provisioning', image: '', showOnResourcePanel: true},
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {maxPerPrompt: 5},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const props = (result[0].data as {properties?: Record<string, unknown>})?.properties;

      expect(props?.maxPerPrompt).toBe('not-a-number');
    });
  });

  describe('Executor Edge Cases', () => {
    it('should handle executor without matching name', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'step-1',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'NonExistentExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            display: {label: 'Different Executor', image: '', showOnResourcePanel: true},
            data: {
              action: {executor: {name: 'DifferentExecutor'}},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('Step Label');
    });
  });
});
