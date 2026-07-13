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

import {IdentityProviderTypes, type BasicIdentityProvider} from '@thunderid/configure-connections';
import {describe, expect, it} from 'vitest';
import {ExecutionTypes} from '../../models/steps';
import computeExecutorConnections from '../computeExecutorConnections';
import type {NotificationSender} from '@/features/notification-senders/models/notification-sender';

describe('computeExecutorConnections', () => {
  const createIdp = (id: string, type: BasicIdentityProvider['type'], name = 'Test IDP'): BasicIdentityProvider => ({
    id,
    name,
    type,
  });

  const createNotificationSender = (id: string, name = 'Test Sender'): NotificationSender => ({
    id,
    name,
    provider: 'twilio',
  });

  describe('Identity Providers', () => {
    it('should map Google IDP to GoogleOIDCAuthExecutor', () => {
      const idps = [createIdp('google-1', IdentityProviderTypes.GOOGLE)];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(1);
      expect(result[0].executorName).toBe(ExecutionTypes.GoogleFederation);
      expect(result[0].connections).toEqual(['google-1']);
    });

    it('should map GitHub IDP to GithubOAuthExecutor', () => {
      const idps = [createIdp('github-1', IdentityProviderTypes.GITHUB)];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(1);
      expect(result[0].executorName).toBe(ExecutionTypes.GithubFederation);
      expect(result[0].connections).toEqual(['github-1']);
    });

    it('should group multiple IDPs of the same type', () => {
      const idps = [
        createIdp('google-1', IdentityProviderTypes.GOOGLE, 'Google 1'),
        createIdp('google-2', IdentityProviderTypes.GOOGLE, 'Google 2'),
      ];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(1);
      expect(result[0].executorName).toBe(ExecutionTypes.GoogleFederation);
      expect(result[0].connections).toEqual(['google-1', 'google-2']);
    });

    it('should handle multiple IDP types', () => {
      const idps = [
        createIdp('google-1', IdentityProviderTypes.GOOGLE),
        createIdp('github-1', IdentityProviderTypes.GITHUB),
      ];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(2);

      const googleConnection = result.find((c) => c.executorName === ExecutionTypes.GoogleFederation);
      const githubConnection = result.find((c) => c.executorName === ExecutionTypes.GithubFederation);

      expect(googleConnection?.connections).toEqual(['google-1']);
      expect(githubConnection?.connections).toEqual(['github-1']);
    });

    it('should ignore unsupported IDP types', () => {
      const idps = [createIdp('oauth-1', IdentityProviderTypes.OAUTH), createIdp('oidc-1', IdentityProviderTypes.OIDC)];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(0);
    });

    it('should mix supported and unsupported IDP types', () => {
      const idps = [
        createIdp('google-1', IdentityProviderTypes.GOOGLE),
        createIdp('oauth-1', IdentityProviderTypes.OAUTH),
        createIdp('github-1', IdentityProviderTypes.GITHUB),
        createIdp('oidc-1', IdentityProviderTypes.OIDC),
      ];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(result).toHaveLength(2);
      expect(result.some((c) => c.executorName === ExecutionTypes.GoogleFederation)).toBe(true);
      expect(result.some((c) => c.executorName === ExecutionTypes.GithubFederation)).toBe(true);
    });
  });

  describe('Notification Senders', () => {
    it('should map notification senders to SMSExecutor', () => {
      const senders = [createNotificationSender('sender-1')];

      const result = computeExecutorConnections({notificationSenders: senders});

      expect(result).toHaveLength(1);
      expect(result[0].executorName).toBe(ExecutionTypes.SMSExecutor);
      expect(result[0].connections).toEqual(['sender-1']);
    });

    it('should include all sender IDs in connections', () => {
      const senders = [
        createNotificationSender('sender-1', 'Twilio'),
        createNotificationSender('sender-2', 'Vonage'),
        createNotificationSender('sender-3', 'Custom'),
      ];

      const result = computeExecutorConnections({notificationSenders: senders});

      expect(result).toHaveLength(1);
      expect(result[0].executorName).toBe(ExecutionTypes.SMSExecutor);
      expect(result[0].connections).toEqual(['sender-1', 'sender-2', 'sender-3']);
    });
  });

  describe('Combined IDPs and Notification Senders', () => {
    it('should process both IDPs and notification senders', () => {
      const idps = [createIdp('google-1', IdentityProviderTypes.GOOGLE)];
      const senders = [createNotificationSender('sender-1')];

      const result = computeExecutorConnections({
        identityProviders: idps,
        notificationSenders: senders,
      });

      expect(result).toHaveLength(2);

      const googleConnection = result.find((c) => c.executorName === ExecutionTypes.GoogleFederation);
      const smsConnection = result.find((c) => c.executorName === ExecutionTypes.SMSExecutor);

      expect(googleConnection?.connections).toEqual(['google-1']);
      expect(smsConnection?.connections).toEqual(['sender-1']);
    });

    it('should handle all supported types together', () => {
      const idps = [
        createIdp('google-1', IdentityProviderTypes.GOOGLE),
        createIdp('google-2', IdentityProviderTypes.GOOGLE),
        createIdp('github-1', IdentityProviderTypes.GITHUB),
      ];
      const senders = [createNotificationSender('sender-1'), createNotificationSender('sender-2')];

      const result = computeExecutorConnections({
        identityProviders: idps,
        notificationSenders: senders,
      });

      expect(result).toHaveLength(3);

      const googleConnection = result.find((c) => c.executorName === ExecutionTypes.GoogleFederation);
      const githubConnection = result.find((c) => c.executorName === ExecutionTypes.GithubFederation);
      const smsConnection = result.find((c) => c.executorName === ExecutionTypes.SMSExecutor);

      expect(googleConnection?.connections).toEqual(['google-1', 'google-2']);
      expect(githubConnection?.connections).toEqual(['github-1']);
      expect(smsConnection?.connections).toEqual(['sender-1', 'sender-2']);
    });
  });

  describe('Edge Cases', () => {
    it('should return empty array when no params provided', () => {
      const result = computeExecutorConnections({});

      expect(result).toEqual([]);
    });

    it('should return empty array when identityProviders is undefined', () => {
      const result = computeExecutorConnections({identityProviders: undefined});

      expect(result).toEqual([]);
    });

    it('should return empty array when notificationSenders is undefined', () => {
      const result = computeExecutorConnections({notificationSenders: undefined});

      expect(result).toEqual([]);
    });

    it('should return empty array when identityProviders is empty', () => {
      const result = computeExecutorConnections({identityProviders: []});

      expect(result).toEqual([]);
    });

    it('should return empty array when notificationSenders is empty', () => {
      const result = computeExecutorConnections({notificationSenders: []});

      expect(result).toEqual([]);
    });

    it('should handle empty arrays for both params', () => {
      const result = computeExecutorConnections({
        identityProviders: [],
        notificationSenders: [],
      });

      expect(result).toEqual([]);
    });
  });

  describe('Return Format', () => {
    it('should return array of ExecutorConnectionInterface objects', () => {
      const idps = [createIdp('google-1', IdentityProviderTypes.GOOGLE)];

      const result = computeExecutorConnections({identityProviders: idps});

      expect(Array.isArray(result)).toBe(true);
      expect(result[0]).toHaveProperty('executorName');
      expect(result[0]).toHaveProperty('connections');
      expect(Array.isArray(result[0].connections)).toBe(true);
    });

    it('should preserve order based on Map iteration', () => {
      const idps = [
        createIdp('google-1', IdentityProviderTypes.GOOGLE),
        createIdp('github-1', IdentityProviderTypes.GITHUB),
        createIdp('google-2', IdentityProviderTypes.GOOGLE),
      ];

      const result = computeExecutorConnections({identityProviders: idps});

      // Map preserves insertion order, so Google should come first
      expect(result[0].executorName).toBe(ExecutionTypes.GoogleFederation);
      expect(result[1].executorName).toBe(ExecutionTypes.GithubFederation);
    });
  });
});
