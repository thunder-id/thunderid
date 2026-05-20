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

import {BrandingPreference, Theme} from '@thunderid/browser';
import {Context, createContext} from 'react';

export interface BrandingContextValue {
  /**
   * The active theme mode from branding preference ('light' | 'dark')
   */
  activeTheme: 'light' | 'dark' | null;
  /**
   * The raw branding preference data
   */
  brandingPreference: BrandingPreference | null;
  /**
   * Error state
   */
  error: Error | null;
  /**
   * Function to manually fetch branding preference
   */
  fetchBranding: () => Promise<void>;
  /**
   * Loading state
   */
  isLoading: boolean;
  /**
   * Function to refetch branding preference
   * This bypasses the single-call restriction and forces a new API call
   */
  refetch: () => Promise<void>;
  /**
   * The transformed theme object
   */
  theme: Theme | null;
}

const BrandingContext: Context<BrandingContextValue | null> = createContext<BrandingContextValue | null>(null);

BrandingContext.displayName = 'BrandingContext';

export default BrandingContext;
