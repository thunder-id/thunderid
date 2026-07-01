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
import type {Application, BasicApplication} from '../application';

describe('Application Models', () => {
  describe('BasicApplication', () => {
    it('should have required id and name properties', () => {
      const app: BasicApplication = {
        id: '550e8400-e29b-41d4-a716-446655440000',
        name: 'Test Application',
      };

      expect(app.id).toBe('550e8400-e29b-41d4-a716-446655440000');
      expect(app.name).toBe('Test Application');
    });

    it('should accept optional properties', () => {
      const app: BasicApplication = {
        id: '1',
        name: 'Test App',
        description: 'Test description',
        logoUrl: 'https://example.com/logo.png',
        clientId: 'test_client_id',
        authFlowId: 'auth_flow_123',
        registrationFlowId: 'reg_flow_456',
        isRegistrationFlowEnabled: true,
        template: 'react',
      };

      expect(app.description).toBe('Test description');
      expect(app.logoUrl).toBe('https://example.com/logo.png');
      expect(app.clientId).toBe('test_client_id');
      expect(app.authFlowId).toBe('auth_flow_123');
      expect(app.registrationFlowId).toBe('reg_flow_456');
      expect(app.isRegistrationFlowEnabled).toBe(true);
      expect(app.template).toBe('react');
    });
  });

  describe('Application', () => {
    it('should accept minimal application object', () => {
      const app: Application = {
        id: '1',
        name: 'Test App',
        inboundAuthConfig: [],
      };

      expect(app.id).toBe('1');
      expect(app.name).toBe('Test App');
      expect(app.inboundAuthConfig).toEqual([]);
    });

    it('should accept full application object with all properties', () => {
      const app: Application = {
        id: '550e8400-e29b-41d4-a716-446655440000',
        name: 'My Web Application',
        description: 'Customer portal application',
        url: 'https://myapp.com',
        logoUrl: 'https://myapp.com/logo.png',
        tosUri: 'https://myapp.com/terms',
        policyUri: 'https://myapp.com/privacy',
        contacts: ['admin@myapp.com'],
        authFlowId: 'flow_123',
        registrationFlowId: 'reg_123',
        isRegistrationFlowEnabled: true,
        template: 'nextjs',
        inboundAuthConfig: [],
        themeId: 'theme_123',
        allowedUserTypes: ['INTERNAL'],
      };

      expect(app).toHaveProperty('id');
      expect(app).toHaveProperty('name');
      expect(app).toHaveProperty('description');
      expect(app).toHaveProperty('url');
      expect(app).toHaveProperty('logoUrl');
      expect(app).toHaveProperty('tosUri');
      expect(app).toHaveProperty('policyUri');
      expect(app).toHaveProperty('contacts');
      expect(app).toHaveProperty('authFlowId');
      expect(app).toHaveProperty('registrationFlowId');
      expect(app).toHaveProperty('isRegistrationFlowEnabled');
      expect(app).toHaveProperty('template');
      expect(app).toHaveProperty('inboundAuthConfig');
      expect(app).toHaveProperty('themeId');
      expect(app).toHaveProperty('allowedUserTypes');
    });
  });
});
