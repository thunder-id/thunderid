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
import FlowContext, {FlowContextValue} from './FlowContext';

/**
 * Hook to access the flow context.
 * Must be used within a FlowProvider.
 *
 * @example
 * ```tsx
 * const MyAuthComponent = () => {
 *   const { title, setTitle, addMessage, isLoading } = useFlow();
 *
 *   const handleSuccess = () => {
 *     addMessage({
 *       type: 'success',
 *       message: 'Authentication successful!'
 *     });
 *   };
 *
 *   return (
 *     <div>
 *       <h1>{title}</h1>
 *       {isLoading && <p>Loading...</p>}
 *     </div>
 *   );
 * };
 * ```
 *
 * @returns The flow context value
 * @throws Error if used outside of FlowProvider
 */
const useFlow = (): FlowContextValue => {
  const context: FlowContextValue | undefined = useContext(FlowContext);

  if (!context) {
    throw new Error('useFlow must be used within a FlowProvider');
  }

  return context;
};

export default useFlow;
