/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {describe, expect, it, vi} from 'vitest';
import type {IntegrationGuides} from '../../models/application-templates';

import getIntegrationGuidesForTemplate, {
  getIntegrationGuideForTemplate,
  getIntegrationGuideVariantKey,
} from '../getIntegrationGuidesForTemplate';

// Mock the config files - must be defined before any imports that use them
vi.mock('../../config/TechnologyBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'express',
        integrationGuides: {
          INBUILT: {
            llm_prompt: {
              id: 'express-llm',
              title: 'AI-Assisted Integration',
              description: 'Express integration guide',
              type: 'llm' as const,
              icon: 'express',
              content: 'LLM prompt content',
            },
            manual_steps: [
              {
                step: 1,
                title: 'Step E1',
                description: 'First step description',
              },
              {
                step: 2,
                title: 'Step E2',
                description: 'Second step description',
              },
            ],
          },
        },
      },
    },
    {
      template: {
        id: 'react',
        integrationGuides: {
          INBUILT: {
            llm_prompt: {
              id: 'react-llm',
              title: 'AI-Assisted Integration',
              description: 'React integration guide',
              type: 'llm' as const,
              icon: 'react',
              content: 'LLM prompt content',
            },
            manual_steps: [
              {
                step: 1,
                title: 'Step 1',
                description: 'First step description',
              },
              {
                step: 2,
                title: 'Step 2',
                description: 'Second step description',
              },
            ],
          },
          EMBEDDED: {
            llm_prompt: {
              id: 'react-embedded-llm',
              title: 'AI-Assisted Integration',
              description: 'React embedded integration guide',
              type: 'llm' as const,
              icon: 'react',
              content: 'Embedded LLM prompt content',
            },
            manual_steps: [
              {
                step: 1,
                title: 'Embedded Step 1',
                description: 'First embedded step description',
              },
            ],
          },
        },
      },
    },
    {
      template: {
        id: 'nextjs',
        integrationGuides: {
          INBUILT: {
            llm_prompt: {
              id: 'nextjs-llm',
              title: 'AI-Assisted Integration',
              description: 'Next.js integration guide',
              type: 'llm' as const,
              icon: 'nextjs',
              content: 'LLM prompt content',
            },
            manual_steps: [
              {
                step: 1,
                title: 'Step A',
                description: 'First step description',
              },
              {
                step: 2,
                title: 'Step B',
                description: 'Second step description',
              },
            ],
          },
        },
      },
    },
    {
      template: {
        id: 'angular',
      },
    },
  ],
}));

vi.mock('../../config/PlatformBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'browser',
        integrationGuides: {
          INBUILT: {
            llm_prompt: {
              id: 'browser-llm',
              title: 'AI-Assisted Integration',
              description: 'Browser integration guide',
              type: 'llm' as const,
              icon: 'browser',
              content: 'LLM prompt content',
            },
            manual_steps: [
              {
                step: 1,
                title: 'Step X',
                description: 'First step description',
              },
              {
                step: 2,
                title: 'Step Y',
                description: 'Second step description',
              },
            ],
          },
        },
      },
    },
    {
      template: {
        id: 'mobile',
      },
    },
  ],
}));

vi.mock('../normalizeTemplateId', () => ({
  default: vi.fn((id: string | undefined) => {
    if (!id) return id;
    return id.replace('-embedded', '');
  }),
}));

// Test data - define after mocks
const mockReactGuides: IntegrationGuides = {
  INBUILT: {
    llm_prompt: {
      id: 'react-llm',
      title: 'AI-Assisted Integration',
      description: 'React integration guide',
      type: 'llm' as const,
      icon: 'react',
      content: 'LLM prompt content',
    },
    manual_steps: [
      {
        step: 1,
        title: 'Step 1',
        description: 'First step description',
      },
      {
        step: 2,
        title: 'Step 2',
        description: 'Second step description',
      },
    ],
  },
  EMBEDDED: {
    llm_prompt: {
      id: 'react-embedded-llm',
      title: 'AI-Assisted Integration',
      description: 'React embedded integration guide',
      type: 'llm' as const,
      icon: 'react',
      content: 'Embedded LLM prompt content',
    },
    manual_steps: [
      {
        step: 1,
        title: 'Embedded Step 1',
        description: 'First embedded step description',
      },
    ],
  },
};

const mockExpressGuides: IntegrationGuides = {
  INBUILT: {
    llm_prompt: {
      id: 'express-llm',
      title: 'AI-Assisted Integration',
      description: 'Express integration guide',
      type: 'llm' as const,
      icon: 'express',
      content: 'LLM prompt content',
    },
    manual_steps: [
      {
        step: 1,
        title: 'Step E1',
        description: 'First step description',
      },
      {
        step: 2,
        title: 'Step E2',
        description: 'Second step description',
      },
    ],
  },
};

const mockNextjsGuides: IntegrationGuides = {
  INBUILT: {
    llm_prompt: {
      id: 'nextjs-llm',
      title: 'AI-Assisted Integration',
      description: 'Next.js integration guide',
      type: 'llm' as const,
      icon: 'nextjs',
      content: 'LLM prompt content',
    },
    manual_steps: [
      {
        step: 1,
        title: 'Step A',
        description: 'First step description',
      },
      {
        step: 2,
        title: 'Step B',
        description: 'Second step description',
      },
    ],
  },
};

const mockBrowserGuides: IntegrationGuides = {
  INBUILT: {
    llm_prompt: {
      id: 'browser-llm',
      title: 'AI-Assisted Integration',
      description: 'Browser integration guide',
      type: 'llm' as const,
      icon: 'browser',
      content: 'LLM prompt content',
    },
    manual_steps: [
      {
        step: 1,
        title: 'Step X',
        description: 'First step description',
      },
      {
        step: 2,
        title: 'Step Y',
        description: 'Second step description',
      },
    ],
  },
};

describe('getIntegrationGuidesForTemplate', () => {
  describe('Technology-Based Templates', () => {
    it('should return integration guides for express template', () => {
      const result = getIntegrationGuidesForTemplate('express');

      expect(result).toEqual(mockExpressGuides);
    });

    it('should return integration guides for react template', () => {
      const result = getIntegrationGuidesForTemplate('react');

      expect(result).toEqual(mockReactGuides);
    });

    it('should return integration guides for nextjs template', () => {
      const result = getIntegrationGuidesForTemplate('nextjs');

      expect(result).toEqual(mockNextjsGuides);
    });

    it('should return null for angular template with no integration guides', () => {
      const result = getIntegrationGuidesForTemplate('angular');

      expect(result).toBeNull();
    });
  });

  describe('Platform-Based Templates', () => {
    it('should return integration guides for browser template', () => {
      const result = getIntegrationGuidesForTemplate('browser');

      expect(result).toEqual(mockBrowserGuides);
    });

    it('should return null for mobile template with no integration guides', () => {
      const result = getIntegrationGuidesForTemplate('mobile');

      expect(result).toBeNull();
    });
  });

  describe('Embedded Templates', () => {
    it('should return integration guides for react-embedded by normalizing to react', () => {
      const result = getIntegrationGuidesForTemplate('react-embedded');

      expect(result).toEqual(mockReactGuides);
    });

    it('should return integration guides for nextjs-embedded by normalizing to nextjs', () => {
      const result = getIntegrationGuidesForTemplate('nextjs-embedded');

      expect(result).toEqual(mockNextjsGuides);
    });
  });

  describe('Edge Cases', () => {
    it('should return null for undefined template ID', () => {
      const result = getIntegrationGuidesForTemplate(undefined);

      expect(result).toBeNull();
    });

    it('should return null for empty string template ID', () => {
      const result = getIntegrationGuidesForTemplate('');

      expect(result).toBeNull();
    });

    it('should return null for non-existent template ID', () => {
      const result = getIntegrationGuidesForTemplate('non-existent-template');

      expect(result).toBeNull();
    });
  });
});

describe('getIntegrationGuideVariantKey', () => {
  it('should resolve INBUILT for a template without the embedded suffix', () => {
    expect(getIntegrationGuideVariantKey('react')).toBe('INBUILT');
  });

  it('should resolve EMBEDDED for a template with the embedded suffix', () => {
    expect(getIntegrationGuideVariantKey('react-embedded')).toBe('EMBEDDED');
  });

  it('should resolve INBUILT for undefined and null template IDs', () => {
    expect(getIntegrationGuideVariantKey(undefined)).toBe('INBUILT');
    expect(getIntegrationGuideVariantKey(null)).toBe('INBUILT');
  });
});

describe('getIntegrationGuideForTemplate', () => {
  it('should return the INBUILT guide for the react template', () => {
    expect(getIntegrationGuideForTemplate('react')).toEqual(mockReactGuides.INBUILT);
  });

  it('should return the EMBEDDED guide for the react-embedded template', () => {
    expect(getIntegrationGuideForTemplate('react-embedded')).toEqual(mockReactGuides.EMBEDDED);
  });

  it('should return the INBUILT guide for the express template', () => {
    expect(getIntegrationGuideForTemplate('express')).toEqual(mockExpressGuides.INBUILT);
  });

  it('should return null for express-embedded since no EMBEDDED variant exists', () => {
    expect(getIntegrationGuideForTemplate('express-embedded')).toBeNull();
  });

  it('should return null for a template without any integration guides', () => {
    expect(getIntegrationGuideForTemplate('angular')).toBeNull();
  });

  it('should return null for undefined template ID', () => {
    expect(getIntegrationGuideForTemplate(undefined)).toBeNull();
  });

  it('should return null for a non-existent template ID', () => {
    expect(getIntegrationGuideForTemplate('non-existent-template')).toBeNull();
  });
});
