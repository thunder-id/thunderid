// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
          REDIRECT_BASED: {
            llm_prompt: {
              docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
            },
          },
        },
      },
    },
    {
      template: {
        id: 'react',
        integrationGuides: {
          REDIRECT_BASED: {
            llm_prompt: {
              docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
            },
          },
          EMBEDDED: {
            llm_prompt: {
              docsUrl: 'https://thunderid.dev/prompts/embedded.txt',
            },
          },
        },
      },
    },
    {
      template: {
        id: 'nextjs',
        integrationGuides: {
          REDIRECT_BASED: {
            llm_prompt: {
              docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
            },
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
          REDIRECT_BASED: {
            llm_prompt: {
              docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
            },
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
  REDIRECT_BASED: {
    llm_prompt: {
      docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
    },
  },
  EMBEDDED: {
    llm_prompt: {
      docsUrl: 'https://thunderid.dev/prompts/embedded.txt',
    },
  },
};

const mockExpressGuides: IntegrationGuides = {
  REDIRECT_BASED: {
    llm_prompt: {
      docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
    },
  },
};

const mockNextjsGuides: IntegrationGuides = {
  REDIRECT_BASED: {
    llm_prompt: {
      docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
    },
  },
};

const mockBrowserGuides: IntegrationGuides = {
  REDIRECT_BASED: {
    llm_prompt: {
      docsUrl: 'https://thunderid.dev/prompts/redirect-based.txt',
    },
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
  it('should resolve REDIRECT_BASED for a template without the embedded suffix', () => {
    expect(getIntegrationGuideVariantKey('react')).toBe('REDIRECT_BASED');
  });

  it('should resolve EMBEDDED for a template with the embedded suffix', () => {
    expect(getIntegrationGuideVariantKey('react-embedded')).toBe('EMBEDDED');
  });

  it('should resolve REDIRECT_BASED for undefined and null template IDs', () => {
    expect(getIntegrationGuideVariantKey(undefined)).toBe('REDIRECT_BASED');
    expect(getIntegrationGuideVariantKey(null)).toBe('REDIRECT_BASED');
  });
});

describe('getIntegrationGuideForTemplate', () => {
  it('should return the REDIRECT_BASED guide for the react template', () => {
    expect(getIntegrationGuideForTemplate('react')).toEqual(mockReactGuides.REDIRECT_BASED);
  });

  it('should return the EMBEDDED guide for the react-embedded template', () => {
    expect(getIntegrationGuideForTemplate('react-embedded')).toEqual(mockReactGuides.EMBEDDED);
  });

  it('should return the REDIRECT_BASED guide for the express template', () => {
    expect(getIntegrationGuideForTemplate('express')).toEqual(mockExpressGuides.REDIRECT_BASED);
  });

  it('should fall back to the REDIRECT_BASED guide for express-embedded since no EMBEDDED variant exists', () => {
    // Every technology template should offer a coding-agent prompt regardless of sign-in
    // approach, so a missing variant falls back to whichever one is authored.
    expect(getIntegrationGuideForTemplate('express-embedded')).toEqual(mockExpressGuides.REDIRECT_BASED);
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
