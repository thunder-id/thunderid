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
import {
  FlowTypes,
  type MetadataInterface,
  type ConnectorConfigs,
  type AttributeMetadataInterface,
  type ExecutorConnectionInterface,
  type Claim,
} from '../metadata';

describe('metadata', () => {
  describe('FlowTypes', () => {
    it('should have REGISTRATION type defined', () => {
      expect(FlowTypes.REGISTRATION).toBe('REGISTRATION');
    });

    it('should have PASSWORD_RECOVERY type defined', () => {
      expect(FlowTypes.PASSWORD_RECOVERY).toBe('PASSWORD_RECOVERY');
    });

    it('should have LOGIN type defined', () => {
      expect(FlowTypes.LOGIN).toBe('LOGIN');
    });

    it('should be a constant object with all three flow types', () => {
      expect(Object.keys(FlowTypes)).toHaveLength(3);
      expect(FlowTypes).toHaveProperty('REGISTRATION');
      expect(FlowTypes).toHaveProperty('PASSWORD_RECOVERY');
      expect(FlowTypes).toHaveProperty('LOGIN');
    });
  });

  describe('MetadataInterface', () => {
    it('should allow creating a valid metadata object', () => {
      const metadata: MetadataInterface = {
        flowType: FlowTypes.LOGIN,
        supportedExecutors: ['GoogleOIDCAuthExecutor', 'GithubOAuthExecutor'],
        connectorConfigs: {
          multiAttributeLoginEnabled: true,
          accountVerificationEnabled: false,
        },
        attributeProfile: 'default',
        attributeMetadata: [],
        executorConnections: [],
      };

      expect(metadata.flowType).toBe('LOGIN');
      expect(metadata.supportedExecutors).toHaveLength(2);
      expect(metadata.connectorConfigs.multiAttributeLoginEnabled).toBe(true);
    });

    it('should allow optional supportedFlowCompletionConfigs', () => {
      const metadata: MetadataInterface = {
        flowType: FlowTypes.REGISTRATION,
        supportedExecutors: [],
        connectorConfigs: {
          multiAttributeLoginEnabled: false,
          accountVerificationEnabled: true,
        },
        attributeProfile: 'registration',
        supportedFlowCompletionConfigs: ['EMAIL_VERIFICATION', 'SMS_VERIFICATION'],
        attributeMetadata: [],
        executorConnections: [],
      };

      expect(metadata.supportedFlowCompletionConfigs).toHaveLength(2);
      expect(metadata.supportedFlowCompletionConfigs).toContain('EMAIL_VERIFICATION');
    });
  });

  describe('ConnectorConfigs', () => {
    it('should allow creating connector configs', () => {
      const configs: ConnectorConfigs = {
        multiAttributeLoginEnabled: true,
        accountVerificationEnabled: true,
      };

      expect(configs.multiAttributeLoginEnabled).toBe(true);
      expect(configs.accountVerificationEnabled).toBe(true);
    });

    it('should allow both values to be false', () => {
      const configs: ConnectorConfigs = {
        multiAttributeLoginEnabled: false,
        accountVerificationEnabled: false,
      };

      expect(configs.multiAttributeLoginEnabled).toBe(false);
      expect(configs.accountVerificationEnabled).toBe(false);
    });
  });

  describe('AttributeMetadataInterface', () => {
    it('should allow creating attribute metadata', () => {
      const attributeMetadata: AttributeMetadataInterface = {
        name: 'email',
        claimURI: 'http://wso2.org/claims/emailaddress',
        required: true,
        readOnly: false,
        validators: ['EmailValidator'],
      };

      expect(attributeMetadata.name).toBe('email');
      expect(attributeMetadata.claimURI).toBe('http://wso2.org/claims/emailaddress');
      expect(attributeMetadata.required).toBe(true);
      expect(attributeMetadata.readOnly).toBe(false);
      expect(attributeMetadata.validators).toContain('EmailValidator');
    });

    it('should allow empty validators array', () => {
      const attributeMetadata: AttributeMetadataInterface = {
        name: 'username',
        claimURI: 'http://wso2.org/claims/username',
        required: true,
        readOnly: true,
        validators: [],
      };

      expect(attributeMetadata.validators).toHaveLength(0);
    });
  });

  describe('ExecutorConnectionInterface', () => {
    it('should allow creating executor connection', () => {
      const executorConnection: ExecutorConnectionInterface = {
        executorName: 'GoogleOIDCAuthExecutor',
        connections: ['google-idp-connection'],
      };

      expect(executorConnection.executorName).toBe('GoogleOIDCAuthExecutor');
      expect(executorConnection.connections).toHaveLength(1);
    });

    it('should allow multiple connections', () => {
      const executorConnection: ExecutorConnectionInterface = {
        executorName: 'SMSExecutor',
        connections: ['twilio-sms', 'vonage-sms', 'custom-sms'],
      };

      expect(executorConnection.connections).toHaveLength(3);
    });
  });

  describe('Claim', () => {
    it('should allow creating a claim object', () => {
      const claim: Claim = {
        id: 'claim-1',
        claimURI: 'http://wso2.org/claims/emailaddress',
        description: 'Email address of the user',
        displayOrder: 1,
        multiValued: false,
        dataType: 'string',
        displayName: 'Email',
        readOnly: false,
        regEx: '^[a-zA-Z0-9+_.-]+@[a-zA-Z0-9.-]+$',
        required: true,
        supportedByDefault: true,
      };

      expect(claim.id).toBe('claim-1');
      expect(claim.claimURI).toBe('http://wso2.org/claims/emailaddress');
      expect(claim.displayName).toBe('Email');
      expect(claim.required).toBe(true);
    });

    it('should allow optional dialectURI', () => {
      const claim: Claim = {
        id: 'claim-2',
        claimURI: 'http://wso2.org/claims/givenname',
        dialectURI: 'http://wso2.org/claims',
        description: 'First name',
        displayOrder: 2,
        multiValued: false,
        dataType: 'string',
        displayName: 'First Name',
        readOnly: false,
        regEx: '',
        required: false,
        supportedByDefault: true,
      };

      expect(claim.dialectURI).toBe('http://wso2.org/claims');
    });

    it('should allow optional subAttributes', () => {
      const claim: Claim = {
        id: 'claim-3',
        claimURI: 'http://wso2.org/claims/addresses',
        description: 'User addresses',
        displayOrder: 5,
        multiValued: true,
        dataType: 'complex',
        subAttributes: ['street', 'city', 'country', 'postalCode'],
        displayName: 'Addresses',
        readOnly: false,
        regEx: '',
        required: false,
        supportedByDefault: false,
      };

      expect(claim.subAttributes).toHaveLength(4);
      expect(claim.subAttributes).toContain('city');
    });
  });
});
