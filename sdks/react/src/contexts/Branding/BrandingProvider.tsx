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

import {BrandingPreference, Theme, transformBrandingPreferenceToTheme} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useCallback, useEffect, useState} from 'react';
import BrandingContext from './BrandingContext';

/**
 * Configuration options for the BrandingProvider
 */
export interface BrandingProviderProps {
  /**
   * The branding preference data passed from parent (typically ThunderIDProvider)
   */
  brandingPreference?: BrandingPreference | null;
  /**
   * Whether the branding provider is enabled
   * @default true
   */
  enabled?: boolean;
  /**
   * Error state passed from parent
   */
  error?: Error | null;
  /**
   * Force a specific theme ('light' or 'dark')
   * If not provided, will use the activeTheme from branding preference
   */
  forceTheme?: 'light' | 'dark';
  /**
   * Loading state passed from parent
   */
  isLoading?: boolean;
  /**
   * Function to refetch branding preference passed from parent
   */
  refetch?: () => Promise<void>;
}

/**
 * BrandingProvider component that manages branding state and provides branding context to child components.
 *
 * This provider receives branding preferences from a parent component (typically ThunderIDProvider)
 * and transforms them into theme objects, making them available to all child components.
 *
 * Features:
 * - Receives branding preferences as props
 * - Theme transformation from branding preferences
 * - Loading and error states
 * - Support for custom theme forcing
 *
 * @example
 * Basic usage (typically used within ThunderIDProvider):
 * ```tsx
 * <BrandingProvider
 *   brandingPreference={brandingData}
 *   isLoading={isFetching}
 *   error={fetchError}
 * >
 *   <App />
 * </BrandingProvider>
 * ```
 *
 * @example
 * With custom theme forcing:
 * ```tsx
 * <BrandingProvider
 *   brandingPreference={brandingData}
 *   forceTheme="dark"
 *   enabled={true}
 * >
 *   <App />
 * </BrandingProvider>
 * ```
 */
const BrandingProvider: FC<PropsWithChildren<BrandingProviderProps>> = ({
  children,
  brandingPreference: externalBrandingPreference,
  forceTheme,
  enabled = true,
  isLoading: externalIsLoading = false,
  error: externalError = null,
  refetch: externalRefetch,
}: PropsWithChildren<BrandingProviderProps>): ReactElement => {
  const [theme, setTheme] = useState<Theme | null>(null);
  const [activeTheme, setActiveTheme] = useState<'light' | 'dark' | null>(null);

  // Process branding preference when it changes
  useEffect(() => {
    if (!enabled || !externalBrandingPreference) {
      setTheme(null);
      setActiveTheme(null);
      return;
    }

    // Extract active theme from branding preference
    const activeThemeFromBranding: string | undefined = externalBrandingPreference?.preference?.theme?.activeTheme;
    let extractedActiveTheme: 'light' | 'dark' | null = null;

    if (activeThemeFromBranding) {
      // Convert to lowercase and map to our expected values
      const themeMode: string = activeThemeFromBranding.toLowerCase();
      if (themeMode === 'light' || themeMode === 'dark') {
        extractedActiveTheme = themeMode;
      }
    }

    setActiveTheme(extractedActiveTheme);

    // Transform branding preference to theme
    const transformedTheme: Theme | null = transformBrandingPreferenceToTheme(externalBrandingPreference, forceTheme);
    setTheme(transformedTheme);
  }, [externalBrandingPreference, forceTheme, enabled]);

  // Reset state when disabled
  useEffect(() => {
    if (!enabled) {
      setTheme(null);
      setActiveTheme(null);
    }
  }, [enabled]);

  // Dummy fetchBranding for backward compatibility
  const fetchBranding: () => Promise<void> = useCallback(async (): Promise<void> => {
    if (externalRefetch) {
      await externalRefetch();
    }
  }, [externalRefetch]);

  const value: any = {
    activeTheme,
    brandingPreference: externalBrandingPreference || null,
    error: externalError,
    fetchBranding,
    isLoading: externalIsLoading,
    refetch: externalRefetch || fetchBranding,
    theme,
  };

  return <BrandingContext.Provider value={value}>{children}</BrandingContext.Provider>;
};

export default BrandingProvider;
