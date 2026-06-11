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

import {describe, expect, it} from 'vitest';
import {TechnologyApplicationTemplate, PlatformApplicationTemplate} from '../application-templates';
import type {IntegrationGuide, IntegrationStepCode} from '../application-templates';

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

    it('should have all expected properties', () => {
      const expectedKeys = ['REACT', 'EXPRESS', 'NEXTJS', 'VANILLA_JS', 'VUE', 'NUXT', 'NODEJS', 'OTHER'];

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

    it('should have CUSTOM platform', () => {
      expect(PlatformApplicationTemplate.CUSTOM).toBe('CUSTOM');
    });

    it('should have all expected properties', () => {
      const expectedKeys = ['BACKEND', 'BROWSER', 'MOBILE', 'FULL_STACK', 'CUSTOM'];

      expect(Object.keys(PlatformApplicationTemplate)).toEqual(expectedKeys);
    });
  });

  describe('IntegrationGuide Interface', () => {
    it('should accept valid integration guide with llm type', () => {
      const guide: IntegrationGuide = {
        id: 'guide-1',
        title: 'AI-Assisted Integration',
        description: 'Use AI to integrate your app',
        type: 'llm',
        icon: 'ai-icon',
        content: 'LLM prompt content',
      };

      expect(guide.type).toBe('llm');
      expect(guide.content).toBe('LLM prompt content');
    });

    it('should accept valid integration guide with manual type', () => {
      const guide: IntegrationGuide = {
        id: 'guide-2',
        title: 'Manual Integration',
        description: 'Step-by-step integration',
        type: 'manual',
        icon: 'manual-icon',
      };

      expect(guide.type).toBe('manual');
      expect(guide.content).toBeUndefined();
    });

    it('should have required properties', () => {
      const guide: IntegrationGuide = {
        id: 'test',
        title: 'Test Guide',
        description: 'Test description',
        type: 'manual',
        icon: 'test-icon',
      };

      expect(guide).toHaveProperty('id');
      expect(guide).toHaveProperty('title');
      expect(guide).toHaveProperty('description');
      expect(guide).toHaveProperty('type');
      expect(guide).toHaveProperty('icon');
    });
  });

  describe('IntegrationStepCode Interface', () => {
    it('should accept code block with all properties', () => {
      const codeBlock: IntegrationStepCode = {
        language: 'typescript',
        filename: 'app.ts',
        content: 'const app = "test";',
      };

      expect(codeBlock.language).toBe('typescript');
      expect(codeBlock.filename).toBe('app.ts');
      expect(codeBlock.content).toBe('const app = "test";');
    });

    it('should accept code block without optional filename', () => {
      const codeBlock: IntegrationStepCode = {
        language: 'javascript',
        content: 'console.log("hello");',
      };

      expect(codeBlock.language).toBe('javascript');
      expect(codeBlock.filename).toBeUndefined();
      expect(codeBlock.content).toBe('console.log("hello");');
    });
  });
});
