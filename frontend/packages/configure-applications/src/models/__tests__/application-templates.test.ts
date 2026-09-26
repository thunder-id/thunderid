// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../application-templates';
import type {IntegrationGuide, TemplateCategory} from '../application-templates';

describe('Application Templates Models', () => {
  describe('TechnologyApplicationTemplate', () => {
    it('should have EXPRESS template', () => {
      expect(TechnologyApplicationTemplate.EXPRESS).toBe('EXPRESS');
    });

    it('should have REACT template', () => {
      expect(TechnologyApplicationTemplate.REACT).toBe('REACT');
    });

    it('should have NEXTJS template', () => {
      expect(TechnologyApplicationTemplate.NEXTJS).toBe('NEXTJS');
    });

    it('should have VANILLA_JS template', () => {
      expect(TechnologyApplicationTemplate.VANILLA_JS).toBe('VANILLA_JS');
    });

    it('should have OTHER template', () => {
      expect(TechnologyApplicationTemplate.OTHER).toBe('OTHER');
    });

    it('should have MCP_CLIENT template', () => {
      expect(TechnologyApplicationTemplate.MCP_CLIENT).toBe('MCP_CLIENT');
    });

    it('should have all expected properties', () => {
      const expectedKeys = [
        'REACT',
        'EXPRESS',
        'NEXTJS',
        'VANILLA_JS',
        'VUE',
        'NUXT',
        'NODEJS',
        'IOS',
        'ANDROID',
        'FLUTTER',
        'OTHER',
        'MCP_CLIENT',
      ];

      expect(Object.keys(TechnologyApplicationTemplate)).toEqual(expectedKeys);
    });
  });

  describe('PlatformApplicationTemplate', () => {
    it('should have BACKEND platform', () => {
      expect(PlatformApplicationTemplate.BACKEND).toBe('BACKEND');
    });

    it('should have BROWSER platform', () => {
      expect(PlatformApplicationTemplate.BROWSER).toBe('BROWSER');
    });

    it('should have MOBILE platform', () => {
      expect(PlatformApplicationTemplate.MOBILE).toBe('MOBILE');
    });

    it('should have FULL_STACK platform', () => {
      expect(PlatformApplicationTemplate.FULL_STACK).toBe('FULL_STACK');
    });

    it('should have WALLET platform', () => {
      expect(PlatformApplicationTemplate.WALLET).toBe('WALLET');
    });

    it('should have CUSTOM platform', () => {
      expect(PlatformApplicationTemplate.CUSTOM).toBe('CUSTOM');
    });

    it('should have all expected properties', () => {
      const expectedKeys = ['BACKEND', 'BROWSER', 'MOBILE', 'FULL_STACK', 'WALLET', 'CUSTOM'];

      expect(Object.keys(PlatformApplicationTemplate)).toEqual(expectedKeys);
    });
  });

  describe('TemplateCategory', () => {
    it('should allow "ai" as a template category', () => {
      const category: TemplateCategory = 'ai';

      expect(category).toBe('ai');
    });
  });

  describe('IntegrationGuide Interface', () => {
    it('should accept a docsUrl referencing a documentation.links key', () => {
      const guide: IntegrationGuide = {
        docsUrl: '{{applications.templates.react.llmPrompt.redirectBased}}',
      };

      expect(guide.docsUrl).toBe('{{applications.templates.react.llmPrompt.redirectBased}}');
    });

    it('should accept a literal docsUrl', () => {
      const guide: IntegrationGuide = {
        docsUrl: 'https://thunderid.dev/prompts/react/redirect-based.txt',
      };

      expect(guide.docsUrl).toBe('https://thunderid.dev/prompts/react/redirect-based.txt');
    });

    it('should allow an integration guide with no docsUrl', () => {
      const guide: IntegrationGuide = {};

      expect(guide.docsUrl).toBeUndefined();
    });
  });
});
