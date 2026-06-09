/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {describe, it, expect} from 'vitest';
import type {BasicFlowDefinition} from '../../models/responses';
import findMatchingFlowForIntegrations from '../findMatchingFlowForIntegrations';

describe('findMatchingFlowForIntegrations', () => {
  const createFlow = (id: string, handle: string, name: string): BasicFlowDefinition => ({
    id,
    handle,
    name,
    flowType: 'AUTHENTICATION',
    activeVersion: 1,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  });

  const availableFlows: BasicFlowDefinition[] = [
    createFlow('1', 'basic-flow', 'Basic Auth Flow'),
    createFlow('2', 'google-flow', 'Google Flow'),
    createFlow('3', 'basic-google-flow', 'Basic + Google Flow'),
    createFlow('4', 'basic-github-flow', 'Basic + GitHub Flow'),
    createFlow('5', 'basic-google-github-flow', 'Basic + Google + GitHub Flow'),
    createFlow('6', 'sms-flow', 'SMS OTP Flow'),
    createFlow('7', 'basic-sms-flow', 'Basic + SMS Flow'),
  ];

  describe('Empty Integrations', () => {
    it('should return null when no integrations are enabled', () => {
      const result = findMatchingFlowForIntegrations([], availableFlows);
      expect(result).toBeNull();
    });
  });

  describe('Single Integration Matching', () => {
    it('should match basic auth flow for basic_auth integration', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('basic-flow');
    });

    it('should match google flow for google integration', () => {
      const result = findMatchingFlowForIntegrations(['google'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('google-flow');
    });

    it('should match sms flow for sms-otp integration', () => {
      const result = findMatchingFlowForIntegrations(['sms-otp'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('sms-flow');
    });
  });

  describe('Multiple Integration Matching', () => {
    it('should match basic+google flow when both are enabled', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'google'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('basic-google-flow');
    });

    it('should match basic+github flow when both are enabled', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'github'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('basic-github-flow');
    });

    it('should match basic+google+github flow when all three are enabled', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'google', 'github'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('basic-google-github-flow');
    });

    it('should match basic+sms flow when both are enabled', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'sms-otp'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toBe('basic-sms-flow');
    });
  });

  describe('Integration ID Normalization', () => {
    it('should normalize google-related integrations', () => {
      const result = findMatchingFlowForIntegrations(['google-oauth'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toContain('google');
    });

    it('should normalize github-related integrations', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'github-oauth'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toContain('github');
    });

    it('should normalize sms-related integrations', () => {
      const result = findMatchingFlowForIntegrations(['sms'], availableFlows);
      expect(result).not.toBeNull();
      expect(result?.handle).toContain('sms');
    });
  });

  describe('Exact Match Priority', () => {
    it('should prefer exact match over superset match', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth', 'google'], availableFlows);
      // Should match basic-google-flow (2 integrations) not basic-google-github-flow (3 integrations)
      expect(result?.handle).toBe('basic-google-flow');
    });
  });

  describe('Fallback Matching', () => {
    it('should return null when no exact match exists (strict matching)', () => {
      const limitedFlows: BasicFlowDefinition[] = [createFlow('1', 'basic-google-github-flow', 'Full Flow')];

      const result = findMatchingFlowForIntegrations(['credentials_auth', 'google'], limitedFlows);
      expect(result).toBeNull();
    });

    it('should return null when no flow supports all integrations', () => {
      const result = findMatchingFlowForIntegrations(
        ['credentials_auth', 'google', 'github', 'unknown'],
        availableFlows,
      );
      expect(result).toBeNull();
    });
  });

  describe('Edge Cases', () => {
    it('should handle flows without handles', () => {
      const flowsWithNoHandle: BasicFlowDefinition[] = [
        {
          id: '1',
          handle: '',
          name: 'No Handle Flow',
          flowType: 'AUTHENTICATION',
          activeVersion: 1,
          createdAt: '2025-01-01',
          updatedAt: '2025-01-01',
        },
      ];

      const result = findMatchingFlowForIntegrations(['credentials_auth'], flowsWithNoHandle);
      expect(result).toBeNull();
    });

    it('should handle empty flows array', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth'], []);
      expect(result).toBeNull();
    });

    it('should preserve flow metadata in result', () => {
      const result = findMatchingFlowForIntegrations(['credentials_auth'], availableFlows);
      expect(result).toMatchObject({
        id: '1',
        handle: 'basic-flow',
        name: 'Basic Auth Flow',
        flowType: 'AUTHENTICATION',
      });
    });
  });
});
