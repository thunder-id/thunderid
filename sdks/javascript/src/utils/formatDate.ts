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
 * Formats a date string to a human-readable format.
 *
 * @param dateString - The date string to format (optional)
 * @returns A formatted date string in 'Month Day, Year' format, or '-' if no date is provided, or the original string if parsing fails
 *
 * @example
 * ```typescript
 * formatDate('2025-07-09T10:30:00Z'); // Returns "July 9, 2025"
 * formatDate(''); // Returns "-"
 * formatDate(undefined); // Returns "-"
 * formatDate('invalid-date'); // Returns "invalid-date"
 * ```
 */
const formatDate = (dateString?: string): string => {
  if (!dateString) return '-';

  try {
    return new Date(dateString).toLocaleDateString('en-US', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    });
  } catch {
    return dateString;
  }
};

export default formatDate;
