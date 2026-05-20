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

import {useContext} from 'react';
import BrandingContext, {BrandingContextValue} from './BrandingContext';

/**
 * Hook to access the branding context.
 * This hook provides access to branding preferences, theme data, and loading states.
 *
 * @returns The branding context value containing branding preference data, theme, and control functions
 * @throws Error if used outside of a BrandingProvider
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const { theme, activeTheme, isLoading, error } = useBrandingContext();
 *
 *   if (isLoading) return <div>Loading branding...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <div style={{ color: theme?.colors?.primary?.main }}>
 *       <p>Active theme mode: {activeTheme}</p>
 *       <p>Styled with ThunderID branding</p>
 *     </div>
 *   );
 * }
 * ```
 */
const useBrandingContext = (): BrandingContextValue => {
  const context: BrandingContextValue | null = useContext(BrandingContext);
  if (!context) {
    throw new Error('useBrandingContext must be used within a BrandingProvider');
  }
  return context;
};

export default useBrandingContext;
