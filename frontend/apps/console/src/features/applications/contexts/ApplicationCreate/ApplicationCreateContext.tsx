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

import type {Theme} from '@thunderid/design';
import type {Context} from 'react';
import {createContext} from 'react';
import type {BasicFlowDefinition} from '../../../flows/models/responses';
import type {
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowStep,
} from '../../models/application-create-flow';
import type {
  TechnologyApplicationTemplate,
  PlatformApplicationTemplate,
  ApplicationTemplate,
} from '../../models/application-templates';
import type {McpClientType} from '../../models/mcp-client';

/**
 * Application creation context state interface
 *
 * Provides centralized state management for the application creation flow.
 * This interface defines all the state needed across the multi-step onboarding process.
 *
 * @public
 */
export interface ApplicationCreateContextType {
  /**
   * The current step in the application creation flow
   */
  currentStep: ApplicationCreateFlowStep;

  /**
   * Sets the current step in the application creation flow
   */
  setCurrentStep: (step: ApplicationCreateFlowStep) => void;

  /**
   * The name of the application being created
   * @remark Needed for step 01: Application Name collection.
   */
  appName: string;

  /**
   * Sets the name of the application
   * @remark Needed for step 01: Application Name collection.
   */
  setAppName: (name: string) => void;

  /**
   * The selected organization unit ID
   * @remark Needed for step 02: Organization Unit selection.
   */
  ouId: string;

  /**
   * Sets the selected organization unit ID
   * @remark Needed for step 02: Organization Unit selection.
   */
  setOuId: (ouId: string) => void;

  /**
   * The ID of the selected theme
   * @remark Needed for step 02: Application Design.
   */
  themeId: string | null;

  /**
   * Sets the selected theme ID
   * @remark Needed for step 02: Application Design.
   */
  setThemeId: (themeId: string | null) => void;

  /**
   * The selected theme configuration (UI theme data only, not API response wrapper)
   * @remark Needed for step 02: Application Design and preview.
   */
  selectedTheme: Theme | null;

  /**
   * Sets the selected theme configuration
   * @remark Needed for step 02: Application Design and preview.
   */
  setSelectedTheme: (theme: Theme | null) => void;

  /**
   * URL of the selected application logo
   * @remark Needed for step 02: Application Design.
   */
  appLogo: string | null;

  /**
   * Sets the application logo URL
   * @remark Needed for step 02: Application Design.
   */
  setAppLogo: (logo: string | null) => void;

  /**
   * The selected primary color for the application
   * @remark Needed for step 02: Application Design.
   */
  selectedColor: string;

  /**
   * Sets the selected primary color
   * @remark Needed for step 02: Application Design.
   */
  setSelectedColor: (color: string) => void;

  /**
   * Record of enabled authentication integrations
   * Keys are integration IDs, values indicate whether they are enabled.
   * @remark Needed for step 03: Sign-in Options.
   */
  integrations: Record<string, boolean>;

  /**
   * Sets the integrations configuration.
   * @remark Needed for step 03: Sign-in Options.
   */
  setIntegrations: (integrations: Record<string, boolean>) => void;

  /**
   * Toggles an integration on/off
   * @remark Needed for step 03: Sign-in Options.
   */
  toggleIntegration: (integrationId: string) => void;

  /**
   * The selected authentication flow from available flows
   * @remark Needed for step 03: Sign-in Options.
   */
  selectedAuthFlow: BasicFlowDefinition | null;

  /**
   * Sets the selected authentication flow
   * @remark Needed for step 03: Sign-in Options.
   */
  setSelectedAuthFlow: (flow: BasicFlowDefinition | null) => void;

  /**
   * The selected sign-in approach (INBUILT or CUSTOM).
   * @remark Needed for step 04: Configure Approach.
   */
  signInApproach: ApplicationCreateFlowSignInApproach;

  /**
   * Sets the sign-in approach
   * @remark Needed for step 04: Configure Approach.
   */
  setSignInApproach: (approach: ApplicationCreateFlowSignInApproach) => void;

  /**
   * The selected application technology (e.g., react, nextjs, other).
   * @remark Needed for step 05: Technology Stack.
   */
  selectedTechnology: TechnologyApplicationTemplate | null;

  /**
   * Sets the selected technology
   * @remark Needed for step 05: Technology Stack.
   */
  setSelectedTechnology: (technology: TechnologyApplicationTemplate | null) => void;

  /**
   * The selected platform for 'other' technology type
   * @remark Needed for step 05: Technology Stack.
   */
  selectedPlatform: PlatformApplicationTemplate | null;

  /**
   * Sets the selected platform.
   * @remark Needed for step 05: Technology Stack.
   */
  setSelectedPlatform: (platform: PlatformApplicationTemplate | null) => void;

  /**
   * The current selected application template configuration.
   * @remark Needed for step 05: Technology Stack.
   */
  selectedTemplateConfig: ApplicationTemplate | null;

  /**
   * Sets the selected template configuration.
   * @remark Needed for step 05: Technology Stack.
   */
  setSelectedTemplateConfig: (config: ApplicationTemplate | null) => void;

  /**
   * The selected MCP client type (user-delegated or machine-to-machine).
   * @remark Needed for the mcp-client template's Name & Type step.
   */
  mcpClientType: McpClientType;

  /**
   * Sets the selected MCP client type.
   * @remark Needed for the mcp-client template's Name & Type step.
   */
  setMcpClientType: (type: McpClientType) => void;

  /**
   * The redirect URIs configured for the mcp-client template's user-delegated client.
   * @remark Needed for the mcp-client template's Connection step.
   */
  mcpRedirectUris: string[];

  /**
   * Sets the redirect URIs for the mcp-client template's user-delegated client.
   * @remark Needed for the mcp-client template's Connection step.
   */
  setMcpRedirectUris: (uris: string[]) => void;

  /**
   * The hosting URL for web applications.
   * @remark Needed for step 06: Configuration step.
   */
  hostingUrl: string;

  /**
   * Sets the hosting URL.
   * @remark Needed for step 06: Configuration step.
   */
  setHostingUrl: (url: string) => void;

  /**
   * The OAuth callback URL configured in the configure step.
   * @remark Needed for step 06: Configuration step.
   */
  callbackUrlFromConfig: string;

  /**
   * Sets the callback URL from configuration.
   * @remark Needed for step 06: Configuration step.
   */
  setCallbackUrlFromConfig: (url: string) => void;

  /**
   * The relying party ID for Passkey configuration.
   * @remark Needed for step 06: Configuration step.
   */
  relyingPartyId: string;

  /**
   * Sets the relying party ID.
   * @remark Needed for step 06: Configuration step.
   */
  setRelyingPartyId: (id: string) => void;

  /**
   * The relying party name for Passkey configuration.
   * @remark Needed for step 06: Configuration step.
   */
  relyingPartyName: string;

  /**
   * Sets the relying party name.
   * @remark Needed for step 06: Configuration step.
   */
  setRelyingPartyName: (name: string) => void;

  /**
   * Whether the user has completed the onboarding process.
   * Determined by checking if they have any existing applications.
   */
  hasCompletedOnboarding: boolean;

  /**
   * Sets the onboarding completion status
   */
  setHasCompletedOnboarding: (completed: boolean) => void;

  /**
   * Current error message, if any
   */
  error: string | null;

  /**
   * Sets an error message
   */
  setError: (error: string | null) => void;

  /**
   * Resets all state to initial values
   */
  reset: () => void;
}

/**
 * React context for accessing application creation state throughout the application.
 *
 * This context provides access to all the state needed for the multi-step application
 * creation flow. It should be used within an `ApplicationCreateProvider` component.
 *
 * @example
 * ```tsx
 * import ApplicationCreateContext from './ApplicationCreateContext';
 * import { useContext } from 'react';
 *
 * const MyComponent = () => {
 *   const context = useContext(ApplicationCreateContext);
 *   if (!context) {
 *     throw new Error('Component must be used within ApplicationCreateProvider');
 *   }
 *
 *   const { appName, setAppName, currentStep } = context;
 *   return <div>Current step: {currentStep}</div>;
 * };
 * ```
 *
 * @public
 */
const ApplicationCreateContext: Context<ApplicationCreateContextType | undefined> = createContext<
  ApplicationCreateContextType | undefined
>(undefined);

export default ApplicationCreateContext;
