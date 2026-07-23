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
  StepCategories,
  StepTypes,
  StaticStepTypes,
  ExecutionTypes,
  ExecutionStepViewTypes,
  EdgeStyleTypes,
} from '../steps';

describe('steps models', () => {
  describe('StepCategories', () => {
    it('should have Decision category', () => {
      expect(StepCategories.Decision).toBe('DECISION');
    });

    it('should have Interface category', () => {
      expect(StepCategories.Interface).toBe('INTERFACE');
    });

    it('should have Workflow category', () => {
      expect(StepCategories.Workflow).toBe('WORKFLOW');
    });

    it('should have Executor category', () => {
      expect(StepCategories.Executor).toBe('EXECUTOR');
    });

    it('should have exactly 4 categories', () => {
      expect(Object.keys(StepCategories)).toHaveLength(4);
    });
  });

  describe('StepTypes', () => {
    it('should have View type', () => {
      expect(StepTypes.View).toBe('VIEW');
    });

    it('should have Rule type', () => {
      expect(StepTypes.Rule).toBe('RULE');
    });

    it('should have Execution type', () => {
      expect(StepTypes.Execution).toBe('TASK_EXECUTION');
    });

    it('should have End type', () => {
      expect(StepTypes.End).toBe('END');
    });

    it('should have Call type', () => {
      expect(StepTypes.Call).toBe('CALL');
    });

    it('should have exactly 5 step types', () => {
      expect(Object.keys(StepTypes)).toHaveLength(5);
    });
  });

  describe('StaticStepTypes', () => {
    it('should have UserOnboard type', () => {
      expect(StaticStepTypes.UserOnboard).toBe('USER_ONBOARD');
    });

    it('should have Start type', () => {
      expect(StaticStepTypes.Start).toBe('START');
    });

    it('should have exactly 2 static step types', () => {
      expect(Object.keys(StaticStepTypes)).toHaveLength(2);
    });
  });

  describe('ExecutionTypes', () => {
    it('should have GoogleFederation type', () => {
      expect(ExecutionTypes.GoogleFederation).toBe('GoogleOIDCAuthExecutor');
    });

    it('should have GithubFederation type', () => {
      expect(ExecutionTypes.GithubFederation).toBe('GithubOAuthExecutor');
    });

    it('should have OpenID4VPVerify type', () => {
      expect(ExecutionTypes.OpenID4VPVerify).toBe('OpenID4VPVerifyExecutor');
    });

    it('should have OAuthExecutor type', () => {
      expect(ExecutionTypes.OAuthExecutor).toBe('OAuthExecutor');
    });

    it('should have OIDCAuthExecutor type', () => {
      expect(ExecutionTypes.OIDCAuthExecutor).toBe('OIDCAuthExecutor');
    });

    it('should have PasskeyAuth type', () => {
      expect(ExecutionTypes.PasskeyAuth).toBe('PasskeyAuthExecutor');
    });

    it('should have MagicLinkExecutor type', () => {
      expect(ExecutionTypes.MagicLinkExecutor).toBe('MagicLinkExecutor');
    });

    it('should have OTPExecutor type', () => {
      expect(ExecutionTypes.OTPExecutor).toBe('OTPExecutor');
    });

    it('should have ConsentExecutor type', () => {
      expect(ExecutionTypes.ConsentExecutor).toBe('ConsentExecutor');
    });

    it('should have IdentifyingExecutor type', () => {
      expect(ExecutionTypes.IdentifyingExecutor).toBe('IdentifyingExecutor');
    });

    it('should have OUResolverExecutor type', () => {
      expect(ExecutionTypes.OUResolverExecutor).toBe('OUResolverExecutor');
    });

    it('should have InviteExecutor type', () => {
      expect(ExecutionTypes.InviteExecutor).toBe('InviteExecutor');
    });

    it('should have EmailExecutor type', () => {
      expect(ExecutionTypes.EmailExecutor).toBe('EmailExecutor');
    });

    it('should have SMSExecutor type', () => {
      expect(ExecutionTypes.SMSExecutor).toBe('SMSExecutor');
    });

    it('should have CredentialSetter type', () => {
      expect(ExecutionTypes.CredentialSetter).toBe('CredentialSetter');
    });

    it('should have AttributeUniquenessValidator type', () => {
      expect(ExecutionTypes.AttributeUniquenessValidator).toBe('AttributeUniquenessValidator');
    });

    it('should have PermissionValidator type', () => {
      expect(ExecutionTypes.PermissionValidator).toBe('PermissionValidator');
    });

    it('should have ProvisioningExecutor type', () => {
      expect(ExecutionTypes.ProvisioningExecutor).toBe('ProvisioningExecutor');
    });

    it('should have HTTPRequestExecutor type', () => {
      expect(ExecutionTypes.HTTPRequestExecutor).toBe('HTTPRequestExecutor');
    });

    it('should have OUExecutor type', () => {
      expect(ExecutionTypes.OUExecutor).toBe('OUExecutor');
    });

    it('should have UserTypeResolver type', () => {
      expect(ExecutionTypes.UserTypeResolver).toBe('UserTypeResolver');
    });

    it('should have SSOCheck type', () => {
      expect(ExecutionTypes.SSOCheck).toBe('SSOCheckExecutor');
    });

    it('should have Session type', () => {
      expect(ExecutionTypes.Session).toBe('SessionExecutor');
    });

    it('should have AuthAssert type', () => {
      expect(ExecutionTypes.AuthAssert).toBe('AuthAssertExecutor');
    });

    it('should have Authorization type', () => {
      expect(ExecutionTypes.Authorization).toBe('AuthorizationExecutor');
    });

    it('should have exactly 25 execution types', () => {
      expect(Object.keys(ExecutionTypes)).toHaveLength(25);
    });
  });

  describe('ExecutionStepViewTypes', () => {
    it('should have Default view type', () => {
      expect(ExecutionStepViewTypes.Default).toBe('Execution');
    });

    it('should have MagicLinkView type', () => {
      expect(ExecutionStepViewTypes.MagicLinkView).toBe('Magic Link View');
    });

    it('should have PasskeyView type', () => {
      expect(ExecutionStepViewTypes.PasskeyView).toBe('Passkey View');
    });

    it('should have exactly 3 view types', () => {
      expect(Object.keys(ExecutionStepViewTypes)).toHaveLength(3);
    });
  });

  describe('EdgeStyleTypes', () => {
    it('should have Bezier style', () => {
      expect(EdgeStyleTypes.Bezier).toBe('default');
    });

    it('should have SmoothStep style', () => {
      expect(EdgeStyleTypes.SmoothStep).toBe('smoothstep');
    });

    it('should have Step style', () => {
      expect(EdgeStyleTypes.Step).toBe('step');
    });

    it('should have exactly 3 edge styles', () => {
      expect(Object.keys(EdgeStyleTypes)).toHaveLength(3);
    });
  });
});
