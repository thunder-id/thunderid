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

export interface EmbeddedSignInFlowInitiateResponse {
  flowId: string;
  flowStatus: EmbeddedSignInFlowStatus;
  flowType: EmbeddedSignInFlowType;
  links: EmbeddedSignInFlowLink[];
  nextStep: {
    authenticators: EmbeddedSignInFlowAuthenticator[];
    stepType: EmbeddedSignInFlowStepType;
  };
}

export enum EmbeddedSignInFlowStatus {
  FailCompleted = 'FAIL_COMPLETED',
  FailIncomplete = 'FAIL_INCOMPLETE',
  Incomplete = 'INCOMPLETE',
  SuccessCompleted = 'SUCCESS_COMPLETED',
}

export enum EmbeddedSignInFlowType {
  Authentication = 'AUTHENTICATION',
}

export enum EmbeddedSignInFlowStepType {
  AuthenticatorPrompt = 'AUTHENTICATOR_PROMPT',
  MultiOptionsPrompt = 'MULTI_OPTIONS_PROMPT',
}

export interface EmbeddedSignInFlowAuthenticator {
  authenticator: string;
  authenticatorId: string;
  idp: string;
  metadata: {
    i18nKey: string;
    params: {
      confidential: boolean;
      displayName: string;
      i18nKey: string;
      order: number;
      param: string;
      type: EmbeddedSignInFlowAuthenticatorParamType;
    }[];
    promptType: EmbeddedSignInFlowAuthenticatorPromptType;
  };
  requiredParams: string[];
}

export interface EmbeddedSignInFlowLink {
  href: string;
  method: string;
  name: string;
}

export interface EmbeddedSignInFlowHandleRequestPayload {
  flowId: string;
  selectedAuthenticator: {
    authenticatorId: string;
    params: Record<string, string>;
  };
}

export interface EmbeddedSignInFlowHandleResponse {
  authData: Record<string, any>;
  flowStatus: string;
}

export enum EmbeddedSignInFlowAuthenticatorParamType {
  Integer = 'INTEGER',
  MultiValued = 'MULTI_VALUED',
  String = 'STRING',
}

export enum EmbeddedSignInFlowAuthenticatorExtendedParamType {
  Otp = 'OTPCode',
}

export enum EmbeddedSignInFlowAuthenticatorKnownIdPType {
  Local = 'LOCAL',
}

export enum EmbeddedSignInFlowAuthenticatorPromptType {
  /**
   * Prompt for internal system use, such as API keys or tokens.
   */
  InternalPrompt = 'INTERNAL_PROMPT',
  /**
   * Prompt for redirection to another page or service.
   */
  RedirectionPrompt = 'REDIRECTION_PROMPT',
  /**
   * Prompt for user input, typically for username/password or similar credentials.
   */
  UserPrompt = 'USER_PROMPT',
}
