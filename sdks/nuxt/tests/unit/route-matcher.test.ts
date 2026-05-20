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

/* eslint-disable @typescript-eslint/typedef */

import {describe, it, expect} from 'vitest';
import {createRouteMatcher} from '../../src/runtime/utils/createRouteMatcher';

describe('createRouteMatcher', () => {
  describe('literal paths', () => {
    it('matches an exact path', () => {
      const match = createRouteMatcher(['/dashboard']);
      expect(match('/dashboard')).toBe(true);
    });

    it('does not match a different path', () => {
      const match = createRouteMatcher(['/dashboard']);
      expect(match('/settings')).toBe(false);
    });

    it('does not match a sub-path without a wildcard', () => {
      const match = createRouteMatcher(['/dashboard']);
      expect(match('/dashboard/profile')).toBe(false);
    });
  });

  describe('single-segment wildcard (*)', () => {
    it('matches one path segment', () => {
      const match = createRouteMatcher(['/admin/*']);
      expect(match('/admin/users')).toBe(true);
    });

    it('does not match multiple segments', () => {
      const match = createRouteMatcher(['/admin/*']);
      expect(match('/admin/users/1')).toBe(false);
    });

    it('does not match the base path without a segment', () => {
      const match = createRouteMatcher(['/admin/*']);
      expect(match('/admin')).toBe(false);
    });
  });

  describe('deep wildcard (**)', () => {
    it('matches a direct child', () => {
      const match = createRouteMatcher(['/admin/**']);
      expect(match('/admin/users')).toBe(true);
    });

    it('matches a nested path', () => {
      const match = createRouteMatcher(['/admin/**']);
      expect(match('/admin/users/1/edit')).toBe(true);
    });

    it('matches an empty sub-path (base itself)', () => {
      const match = createRouteMatcher(['/admin/**']);
      // /admin/ (trailing slash is acceptable deep match)
      expect(match('/admin/')).toBe(true);
    });
  });

  describe('multiple patterns', () => {
    it('matches any of the provided patterns', () => {
      const match = createRouteMatcher(['/dashboard/**', '/settings']);
      expect(match('/dashboard/profile')).toBe(true);
      expect(match('/settings')).toBe(true);
      expect(match('/about')).toBe(false);
    });
  });

  describe('regex group patterns', () => {
    it('does NOT support /api/(users|posts) alternation — pipe is escaped', () => {
      // The current createRouteMatcher escapes `|` so alternation groups are
      // treated as literal characters. Parens also remain un-escaped.
      // This test documents the known limitation so it is not silently lost.
      // TODO (Phase 1+): decide whether to support alternation or update docs.
      const match = createRouteMatcher(['/api/(users|posts)']);
      // None of these match because the pattern cannot be interpreted as regex alternation
      expect(match('/api/users')).toBe(false);
      expect(match('/api/posts')).toBe(false);
    });
  });

  describe('edge cases', () => {
    it('returns false for an empty pattern list', () => {
      const match = createRouteMatcher([]);
      expect(match('/anything')).toBe(false);
    });

    it('matches the root path "/"', () => {
      const match = createRouteMatcher(['/']);
      expect(match('/')).toBe(true);
      expect(match('/home')).toBe(false);
    });
  });
});
