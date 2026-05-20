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
import UserContext, {UserContextProps} from './UserContext';

/**
 * Hook to access the User context.
 *
 * This hook provides access to user profile data including:
 * - Raw profile API response
 * - SCIM2 schemas
 * - Nested user object
 * - Flattened user profile
 * - Functions to refresh and update user data
 * - Loading states and error handling
 *
 * @returns {UserContextProps} The user context value containing all user-related data and functions
 * @throws {Error} Throws an error if used outside of UserProvider
 *
 * @example
 * ```tsx
 * import {useUser} from '@thunderid/react';
 *
 * function ProfileComponent() {
 *   const {
 *     isLoading,
 *     profile,
 *     schemas,
 *     user,
 *     flattenedUser,
 *     refreshUser,
 *     updateUser,
 *     error
 *   } = useUser();
 *
 *   if (isLoading) {
 *     return <div>Loading user data...</div>;
 *   }
 *
 *   if (error) {
 *     return <div>Error: {error.message}</div>;
 *   }
 *
 *   return (
 *     <div>
 *       <h1>Welcome {user?.name?.givenName}!</h1>
 *       <p>Email: {flattenedUser?.emails}</p>
 *       <button onClick={refreshUser}>Refresh Profile</button>
 *     </div>
 *   );
 * }
 *
 * // Access specific user properties
 * function UserEmail() {
 *   const {flattenedUser} = useUser();
 *   return <span>{flattenedUser?.emails?.[0]}</span>;
 * }
 *
 * // Update user profile
 * function EditProfile() {
 *   const {updateUser, user} = useUser();
 *
 *   const handleUpdate = async () => {
 *     try {
 *       await updateUser({
 *         name: {
 *           givenName: 'John',
 *           familyName: 'Doe'
 *         }
 *       });
 *     } catch (error) {
 *       console.error('Update failed:', error);
 *     }
 *   };
 *
 *   return <button onClick={handleUpdate}>Update Name</button>;
 * }
 * ```
 */
const useUser = (): UserContextProps => {
  const context: UserContextProps | null = useContext(UserContext);

  if (!context) {
    throw new Error('useUser must be used within a UserProvider');
  }

  return context;
};

export default useUser;
