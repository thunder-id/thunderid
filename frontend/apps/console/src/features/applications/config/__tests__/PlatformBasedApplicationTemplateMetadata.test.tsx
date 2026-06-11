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
import {PlatformApplicationTemplate} from '../../models/application-templates';
import PlatformBasedApplicationTemplateMetadata from '../PlatformBasedApplicationTemplateMetadata';

describe('PlatformBasedApplicationTemplateMetadata', () => {
  describe('Structure', () => {
    it('should be an array', () => {
      expect(Array.isArray(PlatformBasedApplicationTemplateMetadata)).toBe(true);
    });

    it('should have exactly 5 platform templates', () => {
      expect(PlatformBasedApplicationTemplateMetadata).toHaveLength(5);
    });

    it('should have all required properties for each template', () => {
      PlatformBasedApplicationTemplateMetadata.forEach((metadata) => {
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

  describe('Browser Platform', () => {
    const browserMetadata = PlatformBasedApplicationTemplateMetadata.find(
      (m) => m.value === PlatformApplicationTemplate.BROWSER,
    );

    it('should exist', () => {
      expect(browserMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(browserMetadata?.value).toBe(PlatformApplicationTemplate.BROWSER);
    });

    it('should have icon component', () => {
      expect(browserMetadata?.icon).toBeDefined();
      const {container} = render(<div>{browserMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(browserMetadata?.titleKey).toBe('applications:onboarding.configure.stack.platform.browser.title');
      expect(browserMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.platform.browser.description',
      );
    });

    it('should have a template', () => {
      expect(browserMetadata?.template).toBeDefined();
      expect(browserMetadata?.template).toHaveProperty(['defaults', 'name']);
    });

    it('should not be disabled', () => {
      expect(browserMetadata?.disabled).toBeUndefined();
    });
  });

  describe('Server Platform', () => {
    const serverMetadata = PlatformBasedApplicationTemplateMetadata.find(
      (m) => m.value === PlatformApplicationTemplate.FULL_STACK,
    );

    it('should exist', () => {
      expect(serverMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(serverMetadata?.value).toBe(PlatformApplicationTemplate.FULL_STACK);
    });

    it('should have icon component', () => {
      expect(serverMetadata?.icon).toBeDefined();
      const {container} = render(<div>{serverMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(serverMetadata?.titleKey).toBe('applications:onboarding.configure.stack.platform.full_stack.title');
      expect(serverMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.platform.full_stack.description',
      );
    });

    it('should have a template', () => {
      expect(serverMetadata?.template).toBeDefined();
      expect(serverMetadata?.template).toHaveProperty(['defaults', 'name']);
    });
  });

  describe('Mobile Platform', () => {
    const mobileMetadata = PlatformBasedApplicationTemplateMetadata.find(
      (m) => m.value === PlatformApplicationTemplate.MOBILE,
    );

    it('should exist', () => {
      expect(mobileMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(mobileMetadata?.value).toBe(PlatformApplicationTemplate.MOBILE);
    });

    it('should have icon component', () => {
      expect(mobileMetadata?.icon).toBeDefined();
      const {container} = render(<div>{mobileMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(mobileMetadata?.titleKey).toBe('applications:onboarding.configure.stack.platform.mobile.title');
      expect(mobileMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.platform.mobile.description',
      );
    });

    it('should have a template', () => {
      expect(mobileMetadata?.template).toBeDefined();
      expect(mobileMetadata?.template).toHaveProperty(['defaults', 'name']);
    });
  });

  describe('Backend Platform', () => {
    const backendMetadata = PlatformBasedApplicationTemplateMetadata.find(
      (m) => m.value === PlatformApplicationTemplate.BACKEND,
    );

    it('should exist', () => {
      expect(backendMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(backendMetadata?.value).toBe(PlatformApplicationTemplate.BACKEND);
    });

    it('should have icon component', () => {
      expect(backendMetadata?.icon).toBeDefined();
      const {container} = render(<div>{backendMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(backendMetadata?.titleKey).toBe('applications:onboarding.configure.stack.platform.backend.title');
      expect(backendMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.platform.backend.description',
      );
    });

    it('should have a template', () => {
      expect(backendMetadata?.template).toBeDefined();
      expect(backendMetadata?.template).toHaveProperty(['defaults', 'name']);
    });
  });

  describe('Custom Platform', () => {
    const customMetadata = PlatformBasedApplicationTemplateMetadata.find(
      (m) => m.value === PlatformApplicationTemplate.CUSTOM,
    );

    it('should exist', () => {
      expect(customMetadata).toBeDefined();
    });

    it('should have correct value', () => {
      expect(customMetadata?.value).toBe(PlatformApplicationTemplate.CUSTOM);
    });

    it('should have icon component', () => {
      expect(customMetadata?.icon).toBeDefined();
      const {container} = render(<div>{customMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(customMetadata?.titleKey).toBe('applications:onboarding.configure.stack.platform.custom.title');
      expect(customMetadata?.descriptionKey).toBe(
        'applications:onboarding.configure.stack.platform.custom.description',
      );
    });

    it('should have a template with name and creationFlow', () => {
      expect(customMetadata?.template).toBeDefined();
      expect(customMetadata?.template.defaults?.name).toBe('My Application');
      expect(customMetadata?.template.creationFlow?.steps).toEqual(['NAME', 'ORGANIZATION_UNIT', 'COMPLETE']);
    });

    it('should have no field constraints', () => {
      expect(customMetadata?.template.fieldConstraints).toBeUndefined();
    });
  });

  describe('Templates', () => {
    it('should have unique values', () => {
      const values = PlatformBasedApplicationTemplateMetadata.map((m) => m.value);
      const uniqueValues = new Set(values);
      expect(uniqueValues.size).toBe(values.length);
    });

    it('should have valid template objects', () => {
      PlatformBasedApplicationTemplateMetadata.forEach((metadata) => {
        expect(metadata.template).toBeDefined();
        expect(metadata.template.defaults?.name).toBeDefined();
        expect(metadata.template.description).toBeDefined();
      });
    });

    it('should all have i18n keys following the naming pattern', () => {
      PlatformBasedApplicationTemplateMetadata.forEach((metadata) => {
        expect(metadata.titleKey).toMatch(/^applications:onboarding\.configure\.stack\.platform\..+\.title$/);
        expect(metadata.descriptionKey).toMatch(
          /^applications:onboarding\.configure\.stack\.platform\..+\.description$/,
        );
      });
    });

    it('should have all enum values represented', () => {
      const configuredValues = PlatformBasedApplicationTemplateMetadata.map((m) => m.value);
      const enumValues = Object.values(PlatformApplicationTemplate);

      enumValues.forEach((enumValue) => {
        expect(configuredValues).toContain(enumValue);
      });
    });

    it('should assign web category to Browser and Full-Stack', () => {
      [PlatformApplicationTemplate.BROWSER, PlatformApplicationTemplate.FULL_STACK].forEach((value) => {
        const meta = PlatformBasedApplicationTemplateMetadata.find((m) => m.value === value);
        expect(meta?.categories).toContain('web');
      });
    });

    it('should assign web and backend categories to Full-Stack', () => {
      const meta = PlatformBasedApplicationTemplateMetadata.find(
        (m) => m.value === PlatformApplicationTemplate.FULL_STACK,
      );
      expect(meta?.categories).toContain('web');
      expect(meta?.categories).toContain('backend');
    });

    it('should assign mobile category to Mobile', () => {
      const meta = PlatformBasedApplicationTemplateMetadata.find((m) => m.value === PlatformApplicationTemplate.MOBILE);
      expect(meta?.categories).toEqual(['mobile']);
    });

    it('should assign backend category to Backend', () => {
      const meta = PlatformBasedApplicationTemplateMetadata.find(
        (m) => m.value === PlatformApplicationTemplate.BACKEND,
      );
      expect(meta?.categories).toEqual(['backend']);
    });

    it('should assign web, backend, and mobile categories to Custom', () => {
      const meta = PlatformBasedApplicationTemplateMetadata.find((m) => m.value === PlatformApplicationTemplate.CUSTOM);
      expect(meta?.categories).toContain('web');
      expect(meta?.categories).toContain('backend');
      expect(meta?.categories).toContain('mobile');
    });
  });

  describe('Icons', () => {
    it('should all have renderable icons', () => {
      PlatformBasedApplicationTemplateMetadata.forEach((metadata) => {
        const {container} = render(<div>{metadata.icon}</div>);
        expect(container.querySelector('svg')).toBeInTheDocument();
      });
    });
  });
});
