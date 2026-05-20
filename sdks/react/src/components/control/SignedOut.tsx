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

import {FC, PropsWithChildren, ReactNode} from 'react';
import useThunderID from '../../contexts/ThunderID/useThunderID';

/**
 * Props for the SignedOut component.
 */
export interface SignedOutProps {
  /**
   * Content to show when the user is signed in.
   */
  fallback?: ReactNode;
}

/**
 * A component that only renders its children when the user is signed out.
 *
 * @remarks This component is only supported in browser based React applications (CSR).
 *
 * @example
 * ```tsx
 * import { SignedOut } from '@thunderid/auth-react';
 *
 * const App = () => {
 *   return (
 *     <SignedOut fallback={<p>You are already signed in</p>}>
 *       <p>Please sign in to continue</p>
 *     </SignedOut>
 *   );
 * }
 * ```
 */
const SignedOut: FC<PropsWithChildren<SignedOutProps>> = ({
  children,
  fallback = null,
}: PropsWithChildren<SignedOutProps>) => {
  const {isSignedIn} = useThunderID();

  if (!isSignedIn) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
};

SignedOut.displayName = 'SignedOut';

export default SignedOut;
