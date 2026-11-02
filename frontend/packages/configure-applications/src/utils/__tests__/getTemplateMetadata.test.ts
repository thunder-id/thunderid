// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import getTemplateMetadata from '../getTemplateMetadata';

// Mock the config files and normalizeTemplateId
vi.mock('../../config/TechnologyBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'react',
        displayName: 'React',
      },
      icon: '<ReactIcon />',
    },
    {
      template: {
        id: 'nextjs',
        displayName: 'Next.js',
      },
      icon: '<NextIcon />',
    },
    {
      template: {
        id: 'angular',
        displayName: 'Angular',
      },
      icon: '<AngularIcon />',
    },
  ],
}));

vi.mock('../../config/PlatformBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'browser',
        displayName: 'Single Page Application',
      },
      icon: '<BrowserIcon />',
    },
    {
      template: {
        id: 'mobile',
        displayName: 'Mobile Application',
      },
      icon: '<MobileIcon />',
    },
  ],
}));

vi.mock('../normalizeTemplateId', () => ({
  default: vi.fn((id: string | undefined) => {
    if (!id) return id;
    return id.replace('-embedded', '');
  }),
}));

describe('getTemplateMetadata', () => {
  describe('Technology-Based Templates', () => {
    it('should return metadata for react template', () => {
      const result = getTemplateMetadata('react');

      expect(result).toEqual({
        icon: '<ReactIcon />',
        displayName: 'React',
      });
    });

    it('should return metadata for nextjs template', () => {
      const result = getTemplateMetadata('nextjs');

      expect(result).toEqual({
        icon: '<NextIcon />',
        displayName: 'Next.js',
      });
    });

    it('should return metadata for angular template', () => {
      const result = getTemplateMetadata('angular');

      expect(result).toEqual({
        icon: '<AngularIcon />',
        displayName: 'Angular',
      });
    });
  });

  describe('Platform-Based Templates', () => {
    it('should return metadata for browser template', () => {
      const result = getTemplateMetadata('browser');

      expect(result).toEqual({
        icon: '<BrowserIcon />',
        displayName: 'Single Page Application',
      });
    });

    it('should return metadata for mobile template', () => {
      const result = getTemplateMetadata('mobile');

      expect(result).toEqual({
        icon: '<MobileIcon />',
        displayName: 'Mobile Application',
      });
    });
  });

  describe('Embedded Templates', () => {
    it('should return metadata for react-embedded template by normalizing to react', () => {
      const result = getTemplateMetadata('react-embedded');

      expect(result).toEqual({
        icon: '<ReactIcon />',
        displayName: 'React',
      });
    });

    it('should return metadata for nextjs-embedded template by normalizing to nextjs', () => {
      const result = getTemplateMetadata('nextjs-embedded');

      expect(result).toEqual({
        icon: '<NextIcon />',
        displayName: 'Next.js',
      });
    });
  });

  describe('Edge Cases', () => {
    it('should return null for undefined template ID', () => {
      const result = getTemplateMetadata(undefined);

      expect(result).toBeNull();
    });

    it('should return null for empty string template ID', () => {
      const result = getTemplateMetadata('');

      expect(result).toBeNull();
    });

    it('should return null for non-existent template ID', () => {
      const result = getTemplateMetadata('non-existent-template');

      expect(result).toBeNull();
    });

    it('should return null when normalizeTemplateId returns empty string', async () => {
      // Import the mock to override its behavior for this test
      const normalizeTemplateId = await import('../normalizeTemplateId');
      vi.mocked(normalizeTemplateId.default).mockReturnValueOnce('');

      const result = getTemplateMetadata('some-template');

      expect(result).toBeNull();
    });
  });
});
