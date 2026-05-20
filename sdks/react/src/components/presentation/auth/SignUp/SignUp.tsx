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

import {Platform} from '@thunderid/browser';
import {FC} from 'react';
import SignUpV1, {SignUpProps as SignUpV1Props} from './v1/SignUp';
import SignUpV2, {SignUpProps as SignUpV2Props} from './v2/SignUp';
import useThunderID from '../../../../contexts/ThunderID/useThunderID';

/**
 * Props for the SignUp component.
 * Extends SignUpV1Props & SignUpV2Props for full compatibility with both React SignUp components.
 */
export type SignUpProps = SignUpV1Props | SignUpV2Props;

/**
 * A styled SignUp component that provides embedded sign-up flow with pre-built styling.
 * This component routes to the appropriate version-specific implementation based on the platform.
 *
 * @example
 * // Default UI
 * ```tsx
 * import { SignUp } from '@thunderid/react';
 *
 * const App = () => {
 *   return (
 *     <SignUp
 *       onSuccess={(response) => {
 *         console.log('Sign-up successful:', response);
 *         // Handle successful sign-up (e.g., redirect, show confirmation)
 *       }}
 *       onError={(error) => {
 *         console.error('Sign-up failed:', error);
 *       }}
 *       onComplete={(redirectUrl) => {
 *         // Platform-specific redirect handling (e.g., Next.js router.push)
 *         router.push(redirectUrl); // or window.location.href = redirectUrl
 *       }}
 *       size="medium"
 *       variant="outlined"
 *       afterSignUpUrl="/welcome"
 *     />
 *   );
 * };
 * ```
 *
 * @example
 * // Custom UI with render props
 * ```tsx
 * import { SignUp } from '@thunderid/react';
 *
 * const App = () => {
 *   return (
 *     <SignUp
 *       onError={(error) => console.error('Error:', error)}
 *       onComplete={(response) => console.log('Success:', response)}
 *     >
 *       {({values, errors, handleInputChange, handleSubmit, isLoading, components}) => (
 *         <div className="custom-signup">
 *           <h1>Custom Sign Up</h1>
 *           {isLoading ? (
 *             <p>Loading...</p>
 *           ) : (
 *             <form onSubmit={(e) => {
 *               e.preventDefault();
 *               handleSubmit(components[0], values);
 *             }}>
 *               <input
 *                 name="username"
 *                 value={values.username || ''}
 *                 onChange={(e) => handleInputChange('username', e.target.value)}
 *               />
 *               {errors.username && <span>{errors.username}</span>}
 *               <button type="submit" disabled={isLoading}>
 *                 {isLoading ? 'Signing up...' : 'Sign Up'}
 *               </button>
 *             </form>
 *           )}
 *         </div>
 *       )}
 *     </SignUp>
 *   );
 * };
 * ```
 */
const SignUp: FC<SignUpProps> = (props: SignUpProps) => {
  const {platform} = useThunderID();

  if (platform === Platform.ThunderID) {
    return <SignUpV2 {...(props as SignUpV2Props)} />;
  }

  return <SignUpV1 {...(props as SignUpV1Props)} />;
};

export default SignUp;
