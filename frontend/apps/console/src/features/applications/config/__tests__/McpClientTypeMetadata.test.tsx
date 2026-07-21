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
import McpClientTypeMetadataList from '../McpClientTypeMetadata';

describe('McpClientTypeMetadataList', () => {
  describe('Structure', () => {
    it('should be an array', () => {
      expect(Array.isArray(McpClientTypeMetadataList)).toBe(true);
    });

    it('should have exactly 2 client types', () => {
      expect(McpClientTypeMetadataList).toHaveLength(2);
    });

    it('should have all required properties for each entry', () => {
      McpClientTypeMetadataList.forEach((metadata) => {
        expect(metadata).toHaveProperty('value');
        expect(metadata).toHaveProperty('icon');
        expect(metadata).toHaveProperty('titleKey');
        expect(metadata).toHaveProperty('descriptionKey');
      });
    });
  });

  describe('User-delegated entry', () => {
    const userDelegatedMetadata = McpClientTypeMetadataList.find((m) => m.value === 'userDelegated');

    it('should exist', () => {
      expect(userDelegatedMetadata).toBeDefined();
    });

    it('should have an icon component', () => {
      const {container} = render(<div>{userDelegatedMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(userDelegatedMetadata?.titleKey).toBe('applications:onboarding.mcp.clientType.userDelegated.title');
      expect(userDelegatedMetadata?.descriptionKey).toBe(
        'applications:onboarding.mcp.clientType.userDelegated.description',
      );
    });
  });

  describe('Machine-to-machine entry', () => {
    const m2mMetadata = McpClientTypeMetadataList.find((m) => m.value === 'm2m');

    it('should exist', () => {
      expect(m2mMetadata).toBeDefined();
    });

    it('should have an icon component', () => {
      const {container} = render(<div>{m2mMetadata?.icon}</div>);
      expect(container.querySelector('svg')).toBeInTheDocument();
    });

    it('should have correct i18n keys', () => {
      expect(m2mMetadata?.titleKey).toBe('applications:onboarding.mcp.clientType.m2m.title');
      expect(m2mMetadata?.descriptionKey).toBe('applications:onboarding.mcp.clientType.m2m.description');
    });
  });
});
