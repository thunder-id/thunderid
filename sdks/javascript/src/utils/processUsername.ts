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

/**
 * Regular expression to match userstore prefixes in usernames.
 * Matches patterns like "DEFAULT/", "ASGARDEO_USER/", "PRIMARY/", etc.
 * The pattern matches any uppercase letters, numbers, and underscores followed by a forward slash.
 */
const USERSTORE_PREFIX_REGEX = /^[A-Z_][A-Z0-9_]*\//;

/**
 * Removes userstore prefixes from a username if they exist.
 * This is commonly used to clean usernames returned from SCIM2 endpoints
 * that include userstore prefixes like "DEFAULT/", "ASGARDEO_USER/", "PRIMARY/", etc.
 *
 * @param username - The username string to process
 * @returns The username without the userstore prefix, or the original username if no prefix exists
 *
 * @example
 * ```typescript
 * const cleanUsername = removeUserstorePrefix("DEFAULT/john.doe");
 * console.log(cleanUsername); // "john.doe"
 *
 * const thunderidUser = removeUserstorePrefix("ASGARDEO_USER/jane.doe");
 * console.log(thunderidUser); // "jane.doe"
 *
 * const primaryUser = removeUserstorePrefix("PRIMARY/admin");
 * console.log(primaryUser); // "admin"
 *
 * const alreadyClean = removeUserstorePrefix("user.name");
 * console.log(alreadyClean); // "user.name"
 *
 * const emptyInput = removeUserstorePrefix("");
 * console.log(emptyInput); // ""
 * ```
 */
export const removeUserstorePrefix = (username?: string): string => {
  if (!username) {
    return '';
  }

  return username.replace(USERSTORE_PREFIX_REGEX, '');
};

/**
 * Processes a user object to remove userstore prefixes from username fields.
 * This is a helper function for processing user objects returned from SCIM2 endpoints.
 * Handles various username field variations: username, userName, and user_name.
 *
 * @param user - The user object to process
 * @returns The user object with processed username fields
 *
 * @example
 * ```typescript
 * const user = { username: "DEFAULT/john.doe", email: "john@example.com" };
 * const processedUser = processUserUsername(user);
 * console.log(processedUser.username); // "john.doe"
 *
 * const camelCaseUser = { userName: "ASGARDEO_USER/jane.doe", email: "jane@example.com" };
 * const processedCamelCaseUser = processUserUsername(camelCaseUser);
 * console.log(processedCamelCaseUser.userName); // "jane.doe"
 *
 * const snakeCaseUser = { user_name: "PRIMARY/admin", email: "admin@example.com" };
 * const processedSnakeCaseUser = processUserUsername(snakeCaseUser);
 * console.log(processedSnakeCaseUser.user_name); // "admin"
 * ```
 */
const processUsername = <T extends {userName?: string; user_name?: string; username?: string}>(user: T): T => {
  if (!user) {
    return user;
  }

  const processedUser: T = {...user};

  // Process username field
  if (processedUser.username) {
    processedUser.username = removeUserstorePrefix(processedUser.username);
  }

  // Process userName field
  if (processedUser.userName) {
    processedUser.userName = removeUserstorePrefix(processedUser.userName);
  }

  // Process user_name field
  if (processedUser.user_name) {
    processedUser.user_name = removeUserstorePrefix(processedUser.user_name);
  }

  return processedUser;
};

export default processUsername;
