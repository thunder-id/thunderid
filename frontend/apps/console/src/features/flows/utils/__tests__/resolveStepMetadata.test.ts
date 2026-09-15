// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

    it('should coerce persisted executor property strings based on executor defaults', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'ProvisioningExecutor'},
            },
            properties: {
              includeOptional: 'true',
              maxPerPrompt: '5',
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {
                executor: {name: 'ProvisioningExecutor'},
              },
              properties: {
                includeOptional: false,
                maxPerPrompt: 0,
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.includeOptional).toBe(true);
      expect(properties.maxPerPrompt).toBe(5);
    });

    it('should coerce the sign-out prompt property string to a boolean based on the executor default', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'session-sign-out-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'SessionSignOutExecutor'},
            },
            properties: {
              promptOnSignOut: 'true',
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {
                executor: {name: 'SessionSignOutExecutor'},
              },
              properties: {
                promptOnSignOut: false,
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].data.properties!.promptOnSignOut).toBe(true);
    });

    it('should fall back to executor defaults for invalid numeric strings in persisted executor properties', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'ProvisioningExecutor'},
            },
            properties: {
              maxPerPrompt: 'Infinity',
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {
                executor: {name: 'ProvisioningExecutor'},
              },
              properties: {
                maxPerPrompt: 0,
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.maxPerPrompt).toBe(0);
    });

    it('should fall back to executor defaults for invalid boolean strings in persisted executor properties', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'ProvisioningExecutor'},
            },
            properties: {
              includeOptional: 'maybe',
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {
                executor: {name: 'ProvisioningExecutor'},
              },
              properties: {
                includeOptional: false,
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.includeOptional).toBe(false);
    });

    it('should fall back to executor default for empty string numeric value in persisted executor properties', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {maxPerPrompt: ''},
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {maxPerPrompt: 3},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.maxPerPrompt).toBe(3);
    });

    it('should coerce "false" string to boolean false in persisted executor properties', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {includeOptional: 'false'},
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {includeOptional: true},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.includeOptional).toBe(false);
    });

    it('should pass through string values when default is also a string', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'provisioning-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {assignGroup: 'my-group'},
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {assignGroup: ''},
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);
      const properties = result[0].data.properties!;

      expect(properties.assignGroup).toBe('my-group');
    });

    it('should resolve executor metadata with mode matching', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'otp-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'OTPExecutor', mode: 'generate'},
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
              label: 'Generate OTP',
              image: '/images/otp-generate.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'OTPExecutor', mode: 'generate'},
              },
            },
          }),
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Verify OTP',
              image: '/images/otp-verify.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'OTPExecutor', mode: 'verify'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('Generate OTP');
      expect(result[0].display.image).toBe('/images/otp-generate.svg');
    });

    it('should resolve executor metadata by the mode node property, not just executor.mode', () => {
      // ProvisioningExecutor ships as a user tile and an agent tile that share a name and differ
      // only by their preset `mode` node property, so the property has to disambiguate them.
      const provisioningTiles = [
        createMockStep({
          type: 'TASK_EXECUTION',
          display: {label: 'Provision User', image: '/images/user.svg', showOnResourcePanel: true},
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {mode: 'user', allowCrossOUProvisioning: false},
          },
        }),
        createMockStep({
          type: 'TASK_EXECUTION',
          display: {label: 'Provision Agent', image: '/images/agent.svg', showOnResourcePanel: true},
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {mode: 'agent'},
          },
        }),
      ];

      const agentStep = resolveStepMetadata(createMockResources({steps: [], executors: provisioningTiles}), [
        createMockStep({
          id: 'provision-agent',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {mode: 'agent'},
          },
        }),
      ]);

      expect(agentStep[0].display.label).toBe('Provision Agent');

      const userStep = resolveStepMetadata(createMockResources({steps: [], executors: provisioningTiles}), [
        createMockStep({
          id: 'provision-user',
          type: 'TASK_EXECUTION',
          data: {
            action: {executor: {name: 'ProvisioningExecutor'}},
            properties: {mode: 'user'},
          },
        }),
      ]);

      expect(userStep[0].display.label).toBe('Provision User');
    });

    it('should resolve a provisioning node saved without a mode to the first tile', () => {
      // Flows authored before the agent tile existed carry no mode and must keep their old tile.
      const result = resolveStepMetadata(
        createMockResources({
          steps: [],
          executors: [
            createMockStep({
              type: 'TASK_EXECUTION',
              display: {label: 'Provision User', image: '/images/user.svg', showOnResourcePanel: true},
              data: {
                action: {executor: {name: 'ProvisioningExecutor'}},
                properties: {mode: 'user'},
              },
            }),
            createMockStep({
              type: 'TASK_EXECUTION',
              display: {label: 'Provision Agent', image: '/images/agent.svg', showOnResourcePanel: true},
              data: {
                action: {executor: {name: 'ProvisioningExecutor'}},
                properties: {mode: 'agent'},
              },
            }),
          ],
        }),
        [
          createMockStep({
            id: 'legacy-provisioning',
            type: 'TASK_EXECUTION',
            data: {
              action: {executor: {name: 'ProvisioningExecutor'}},
              properties: {includeOptional: true},
            },
          }),
        ],
      );

      expect(result[0].display.label).toBe('Provision User');
    });

    it('should fall back to first matching executor when step has no mode', () => {
      const steps: Step[] = [
        createMockStep({
          id: 'otp-step',
          type: 'TASK_EXECUTION',
          data: {
            action: {
              executor: {name: 'OTPExecutor'},
            },
          },
        }),
      ];

      const resources = createMockResources({
        executors: [
          createMockStep({
            type: 'TASK_EXECUTION',
            display: {
              label: 'Generate OTP',
              image: '/images/otp-generate.svg',
              showOnResourcePanel: true,
            },
            data: {
              action: {
                executor: {name: 'OTPExecutor', mode: 'generate'},
              },
            },
          }),
        ],
      });

      const result = resolveStepMetadata(resources, steps);

      expect(result[0].display.label).toBe('Generate OTP');
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
