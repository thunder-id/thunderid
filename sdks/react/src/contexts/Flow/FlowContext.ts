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

import {Context, createContext} from 'react';

/**
 * Types of authentication flows/steps that can be displayed.
 */
export type FlowStep = {
  canGoBack?: boolean;
  id: string;
  metadata?: Record<string, any>;
  subtitle?: string;
  title: string;
  type: 'signin' | 'signup' | 'organization-signin' | 'forgot-password' | 'reset-password' | 'verify-email' | 'mfa';
} | null;

/**
 * Message types for displaying in authentication flows.
 */
export interface FlowMessage {
  dismissible?: boolean;
  id?: string;
  message: string;
  type: 'success' | 'error' | 'warning' | 'info';
}

/**
 * Context value for managing authentication flow UI state.
 */
export interface FlowContextValue {
  addMessage: (message: FlowMessage) => void;
  clearMessages: () => void;

  // Current step/flow
  currentStep: FlowStep;
  // Error state
  error: string | null;
  // Loading state
  isLoading: boolean;
  // Messages
  messages: FlowMessage[];

  navigateToFlow: (
    flowType: NonNullable<FlowStep>['type'],
    options?: {
      metadata?: Record<string, any>;
      subtitle?: string;
      title?: string;
    },
  ) => void;
  onGoBack?: () => void;
  removeMessage: (messageId: string) => void;
  // Utilities
  reset: () => void;

  setCurrentStep: (step: FlowStep) => void;
  setError: (error: string | null) => void;

  setIsLoading: (loading: boolean) => void;
  setOnGoBack: (callback?: () => void) => void;

  setShowBackButton: (show: boolean) => void;
  setSubtitle: (subtitle?: string) => void;
  setTitle: (title: string) => void;

  // Navigation
  showBackButton: boolean;

  subtitle?: string;

  // Title and subtitle
  title: string;
}

/**
 * Context for managing authentication flow UI state.
 * This context handles titles, messages, navigation, and loading states
 * for authentication flows like SignIn, SignUp, organization signin, etc.
 */
const FlowContext: Context<FlowContextValue | undefined> = createContext<FlowContextValue | undefined>(undefined);

FlowContext.displayName = 'FlowContext';

export default FlowContext;
