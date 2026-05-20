/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import RecoveryV1, {RecoveryProps as RecoveryV1Props} from './v1/Recovery';
import RecoveryV2, {RecoveryProps as RecoveryV2Props} from './v2/Recovery';
import useThunderID from '../../../../contexts/ThunderID/useThunderID';

/**
 * Props for the Recovery component.
 * Extends RecoveryV1Props & RecoveryV2Props for full compatibility with both implementations.
 */
export type RecoveryProps = RecoveryV1Props | RecoveryV2Props;

/**
 * Recovery component that provides an embedded account/password recovery flow.
 * Routes to the appropriate version-specific implementation based on the platform.
 *
 * @example
 * ```tsx
 * import { Recovery } from '@thunderid/react';
 *
 * const App = () => (
 *   <Recovery
 *     afterRecoveryUrl="/sign-in"
 *     onComplete={(response) => console.log('Recovery complete', response)}
 *     onError={(error) => console.error('Recovery failed', error)}
 *   />
 * );
 * ```
 *
 * @example
 * // Custom UI with render props
 * ```tsx
 * <Recovery>
 *   {({ values, fieldErrors, handleInputChange, handleSubmit, isLoading, components }) => (
 *     <form onSubmit={(e) => { e.preventDefault(); handleSubmit(components[0], values); }}>
 *       ...
 *     </form>
 *   )}
 * </Recovery>
 * ```
 */
const Recovery: FC<RecoveryProps> = (props: RecoveryProps) => {
  const {platform} = useThunderID();

  if (platform === Platform.ThunderID) {
    return <RecoveryV2 {...(props as RecoveryV2Props)} />;
  }

  return <RecoveryV1 {...(props as RecoveryV1Props)} />;
};

export default Recovery;
