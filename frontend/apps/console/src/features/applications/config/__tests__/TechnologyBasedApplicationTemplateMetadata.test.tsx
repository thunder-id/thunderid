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

import {render} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import {TechnologyApplicationTemplate} from '../../models/application-templates';
import TechnologyBasedApplicationTemplateMetadata from '../TechnologyBasedApplicationTemplateMetadata';

describe('TechnologyBasedApplicationTemplateMetadata', () => {
  describe('Structure', () => {
    it('should be an array', () => {
      expect(Array.isArray(TechnologyBasedApplicationTemplateMetadata)).toBe(true);
    });

    it('should have at least 3 technology templates', () => {
      expect(TechnologyBasedApplicationTemplateMetadata.length).toBeGreaterThanOrEqual(3);
    });

    it('should have all required properties for each template', () => {
      TechnologyBasedApplicationTemplateMetadata.forEach((metadata) => {
        expect(metadata).toHaveProperty('value');
        expect(metadata).toHaveProperty('icon');
        expect(metadata).toHaveProperty('titleKey');
        expect(metadata).toHaveProperty('descriptionKey');
        expect(metadata).toHaveProperty('template');
        expect(metadata).toHaveProperty('categories');
        expect(Array.isArray(metadata.categories)).toBe(true);
        expect(metadata.categories.length).toBeGreaterThan(0);
      });
    });
  });

  describe('Express Technology', () => {
    const expressMetadata = TechnologyBasedApplicationTemplateMetadata.find(
      (m) => m.value === TechnologyApplicationTemplate.EXPRESS,
    );

    it('should exist', () => {
      expect(expressMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(expressMetadata?.value).toBe(TechnologyApplicationTemplate.EXPRESS);
    });

    it('should have icon component', () => {
      expect(expressMetadata?.icon).toBeDefined();
      const {container} = render(<div>{expressMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(expressMetadata?.titleKey).toBe('applications:onboarding.configure.stack.technology.express.title');
      expect(expressMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.technology.express.description',
      );
    });

    it('should have a template', () => {
      expect(expressMetadata?.template).toBeDefined();
      expect(expressMetadata?.template).toHaveProperty(['defaults', 'name']);
    });

    it('should not be disabled', () => {
      expect(expressMetadata?.disabled).not.toBe(true);
    });
  });

  describe('React Technology', () => {
    const reactMetadata = TechnologyBasedApplicationTemplateMetadata.find(
      (m) => m.value === TechnologyApplicationTemplate.REACT,
    );

    it('should exist', () => {
      expect(reactMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(reactMetadata?.value).toBe(TechnologyApplicationTemplate.REACT);
    });

    it('should have icon component', () => {
      expect(reactMetadata?.icon).toBeDefined();
      const {container} = render(<div>{reactMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(reactMetadata?.titleKey).toBe('applications:onboarding.configure.stack.technology.react.title');
      expect(reactMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.technology.react.description',
      );
    });

    it('should have a template', () => {
      expect(reactMetadata?.template).toBeDefined();
      expect(reactMetadata?.template).toHaveProperty(['defaults', 'name']);
    });

    it('should not be disabled', () => {
      expect(reactMetadata?.disabled).not.toBe(true);
    });
  });

  describe('Next.js Technology', () => {
    const nextjsMetadata = TechnologyBasedApplicationTemplateMetadata.find(
      (m) => m.value === TechnologyApplicationTemplate.NEXTJS,
    );

    it('should exist', () => {
      expect(nextjsMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(nextjsMetadata?.value).toBe(TechnologyApplicationTemplate.NEXTJS);
    });

    it('should have icon component', () => {
      expect(nextjsMetadata?.icon).toBeDefined();
      const {container} = render(<div>{nextjsMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(nextjsMetadata?.titleKey).toBe('applications:onboarding.configure.stack.technology.nextjs.title');
      expect(nextjsMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.technology.nextjs.description',
      );
    });

    it('should have a template', () => {
      expect(nextjsMetadata?.template).toBeDefined();
      expect(nextjsMetadata?.template).toHaveProperty(['defaults', 'name']);
    });

    it('should not be disabled', () => {
      expect(nextjsMetadata?.disabled).not.toBe(true);
    });
  });

  describe('Vanilla JavaScript Technology', () => {
    const vanillaJsMetadata = TechnologyBasedApplicationTemplateMetadata.find(
      (m) => m.value === TechnologyApplicationTemplate.VANILLA_JS,
    );

    it('should exist', () => {
      expect(vanillaJsMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(vanillaJsMetadata?.value).toBe(TechnologyApplicationTemplate.VANILLA_JS);
    });

    it('should have icon component', () => {
      expect(vanillaJsMetadata?.icon).toBeDefined();
      const {container} = render(<div>{vanillaJsMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(vanillaJsMetadata?.titleKey).toBe('applications:onboarding.configure.stack.technology.vanillaJs.title');
      expect(vanillaJsMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.technology.vanillaJs.description',
      );
    });

    it('should have a template', () => {
      expect(vanillaJsMetadata?.template).toBeDefined();
      expect(vanillaJsMetadata?.template).toHaveProperty(['defaults', 'name']);
    });

    it('should not be disabled', () => {
      expect(vanillaJsMetadata?.disabled).not.toBe(true);
    });
  });

  describe('Templates', () => {
    it('should have unique values', () => {
      const values = TechnologyBasedApplicationTemplateMetadata.map((m) => m.value);
      const uniqueValues = new Set(values);
      expect(uniqueValues.size).toBe(values.length);
    });

    it('should have valid template objects', () => {
      TechnologyBasedApplicationTemplateMetadata.forEach((metadata) => {
        expect(metadata.template).toBeDefined();
        expect(metadata.template.defaults?.name).toBeDefined();
        expect(metadata.template.description).toBeDefined();
      });
    });

    it('should all have i18n keys following the naming pattern', () => {
      TechnologyBasedApplicationTemplateMetadata.forEach((metadata) => {
        expect(metadata.titleKey).toMatch(/^applications:onboarding\.configure\.stack\.technology\..+\.title$/);
        expect(metadata.descriptionKey).toMatch(
          /^applications:onboarding\.configure\.stack\.technology\..+\.description$/,
        );
      });
    });

    it('should have at least React, Next.js, Vanilla JS, and Express templates', () => {
      const configuredValues = TechnologyBasedApplicationTemplateMetadata.map((m) => m.value);
      expect(configuredValues).toContain(TechnologyApplicationTemplate.EXPRESS);
      expect(configuredValues).toContain(TechnologyApplicationTemplate.REACT);
      expect(configuredValues).toContain(TechnologyApplicationTemplate.NEXTJS);
      expect(configuredValues).toContain(TechnologyApplicationTemplate.VANILLA_JS);
    });

    it('should assign web category to React, Vue, Vanilla JS, and Nuxt', () => {
      const webTemplates = [
        TechnologyApplicationTemplate.REACT,
        TechnologyApplicationTemplate.VUE,
        TechnologyApplicationTemplate.VANILLA_JS,
      ];
      webTemplates.forEach((value) => {
        const meta = TechnologyBasedApplicationTemplateMetadata.find((m) => m.value === value);
        expect(meta?.categories).toContain('web');
      });
    });

    it('should assign web and backend categories to Next.js and Nuxt', () => {
      [TechnologyApplicationTemplate.NEXTJS, TechnologyApplicationTemplate.NUXT].forEach((value) => {
        const meta = TechnologyBasedApplicationTemplateMetadata.find((m) => m.value === value);
        expect(meta?.categories).toContain('web');
        expect(meta?.categories).toContain('backend');
      });
    });

    it('should assign backend category to Express and Node.js', () => {
      [TechnologyApplicationTemplate.EXPRESS, TechnologyApplicationTemplate.NODEJS].forEach((value) => {
        const meta = TechnologyBasedApplicationTemplateMetadata.find((m) => m.value === value);
        expect(meta?.categories).toContain('backend');
      });
    });
  });

  describe('Icons', () => {
    it('should all have renderable icons', () => {
      TechnologyBasedApplicationTemplateMetadata.forEach((metadata) => {
        const {container} = render(<div>{metadata.icon}</div>);
        expect(container.firstChild?.firstChild).toBeTruthy();
      });
    });
  });

  describe('Disabled State', () => {
    it('should have at least one enabled template', () => {
      const enabledTemplates = TechnologyBasedApplicationTemplateMetadata.filter((m) => !m.disabled);
      expect(enabledTemplates.length).toBeGreaterThan(0);
    });

    it('should properly indicate disabled templates', () => {
      const disabledTemplates = TechnologyBasedApplicationTemplateMetadata.filter((m) => m.disabled === true);
      disabledTemplates.forEach((template) => {
        expect(template.disabled).toBe(true);
      });
    });
  });
});
