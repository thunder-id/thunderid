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

import processUsername, {removeUserstorePrefix} from '../processUsername';

describe('processUsername', () => {
  describe('removeUserstorePrefix', () => {
    it('should remove DEFAULT/ prefix from username', () => {
      const result: string = removeUserstorePrefix('DEFAULT/john.doe');
      expect(result).toBe('john.doe');
    });

    it('should remove ASGARDEO_USER/ prefix from username', () => {
      const result: string = removeUserstorePrefix('ASGARDEO_USER/jane.doe');
      expect(result).toBe('jane.doe');
    });

    it('should remove PRIMARY/ prefix from username', () => {
      const result: string = removeUserstorePrefix('PRIMARY/admin');
      expect(result).toBe('admin');
    });

    it('should remove custom userstore prefix from username', () => {
      const result: string = removeUserstorePrefix('CUSTOM_STORE/user.name');
      expect(result).toBe('user.name');
    });

    it('should return original username if no userstore prefix exists', () => {
      const result: string = removeUserstorePrefix('jane.doe');
      expect(result).toBe('jane.doe');
    });

    it('should handle empty string', () => {
      const result: string = removeUserstorePrefix('');
      expect(result).toBe('');
    });

    it('should handle undefined input', () => {
      const result: string = removeUserstorePrefix(undefined);
      expect(result).toBe('');
    });

    it('should handle username with only userstore prefix', () => {
      const result: string = removeUserstorePrefix('DEFAULT/');
      expect(result).toBe('');
    });

    it('should not remove lowercase prefixes', () => {
      const result: string = removeUserstorePrefix('default/user');
      expect(result).toBe('default/user');
    });

    it('should not remove mixed case prefixes', () => {
      const result: string = removeUserstorePrefix('Default/user');
      expect(result).toBe('Default/user');
    });

    it('should not remove if prefix contains invalid characters', () => {
      const result: string = removeUserstorePrefix('DEFAULT-STORE/user');
      expect(result).toBe('DEFAULT-STORE/user');
    });

    it('should only remove the first occurrence of userstore prefix', () => {
      const result: string = removeUserstorePrefix('DEFAULT/DEFAULT/user');
      expect(result).toBe('DEFAULT/user');
    });

    it('should handle userstore prefix with numbers', () => {
      const result: string = removeUserstorePrefix('STORE123/user');
      expect(result).toBe('user');
    });
  });

  describe('processUsername', () => {
    it('should process DEFAULT/ username in user object', () => {
      const user: Record<string, string> = {
        email: 'john@example.com',
        givenName: 'John',
        username: 'DEFAULT/john.doe',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result.username).toBe('john.doe');
      expect(result.email).toBe('john@example.com');
      expect(result.givenName).toBe('John');
    });

    it('should process ASGARDEO_USER/ username in user object', () => {
      const user: Record<string, string> = {
        email: 'jane@example.com',
        givenName: 'Jane',
        username: 'ASGARDEO_USER/jane.doe',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result.username).toBe('jane.doe');
      expect(result.email).toBe('jane@example.com');
      expect(result.givenName).toBe('Jane');
    });

    it('should process PRIMARY/ username in user object', () => {
      const user: Record<string, string> = {
        email: 'admin@example.com',
        givenName: 'Admin',
        username: 'PRIMARY/admin',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result.username).toBe('admin');
      expect(result.email).toBe('admin@example.com');
      expect(result.givenName).toBe('Admin');
    });

    it('should handle user object without username', () => {
      const user: Record<string, string> = {
        email: 'john@example.com',
        givenName: 'John',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result).toEqual(user);
    });

    it('should handle user object with empty username', () => {
      const user: Record<string, string> = {
        email: 'john@example.com',
        username: '',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result.username).toBe('');
      expect(result.email).toBe('john@example.com');
    });

    it('should handle null/undefined user object', () => {
      expect(processUsername(null as any)).toBe(null);
      expect(processUsername(undefined as any)).toBe(undefined);
    });

    it('should preserve other properties in user object', () => {
      const user: Record<string, string> = {
        customProperty: 'customValue',
        email: 'jane@example.com',
        familyName: 'Doe',
        givenName: 'Jane',
        username: 'DEFAULT/jane.doe',
      };

      const result: Record<string, string> = processUsername(user);

      expect(result.username).toBe('jane.doe');
      expect(result.email).toBe('jane@example.com');
      expect(result.givenName).toBe('Jane');
      expect(result.familyName).toBe('Doe');
      expect((result as any).customProperty).toBe('customValue');
    });
  });
});
