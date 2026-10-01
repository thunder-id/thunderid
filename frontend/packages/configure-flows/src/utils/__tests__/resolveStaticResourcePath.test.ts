// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi, beforeEach, afterEach} from 'vitest';
import resolveStaticResourcePath from '../resolveStaticResourcePath';

describe('resolveStaticResourcePath', () => {
  beforeEach(() => {
    vi.stubEnv('BASE_URL', '/app');
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  describe('Basic Functionality', () => {
    it('should prepend BASE_URL to the path', () => {
      const result = resolveStaticResourcePath('images/logo.png');

      expect(result).toBe('/app/images/logo.png');
    });

    it('should handle paths without leading slash', () => {
      const result = resolveStaticResourcePath('assets/icon.svg');

      expect(result).toBe('/app/assets/icon.svg');
    });

    it('should handle paths with leading slash', () => {
      const result = resolveStaticResourcePath('/resources/file.json');

      expect(result).toBe('/app//resources/file.json');
    });
  });

  describe('Different Path Types', () => {
    it('should handle simple file paths', () => {
      const result = resolveStaticResourcePath('file.txt');

      expect(result).toBe('/app/file.txt');
    });

    it('should handle nested directory paths', () => {
      const result = resolveStaticResourcePath('a/b/c/d/file.png');

      expect(result).toBe('/app/a/b/c/d/file.png');
    });

    it('should handle paths with special characters', () => {
      const result = resolveStaticResourcePath('images/my-image_v2.png');

      expect(result).toBe('/app/images/my-image_v2.png');
    });

    it('should handle empty path', () => {
      const result = resolveStaticResourcePath('');

      expect(result).toBe('/app/');
    });
  });

  describe('BASE_URL Variations', () => {
    it('should work with root BASE_URL', () => {
      vi.stubEnv('BASE_URL', '/');
      const result = resolveStaticResourcePath('assets/style.css');

      expect(result).toBe('//assets/style.css');
    });

    it('should work with subdirectory BASE_URL', () => {
      vi.stubEnv('BASE_URL', '/my-app/v2');
      const result = resolveStaticResourcePath('config.json');

      expect(result).toBe('/my-app/v2/config.json');
    });

    it('should work with empty BASE_URL', () => {
      vi.stubEnv('BASE_URL', '');
      const result = resolveStaticResourcePath('file.js');

      expect(result).toBe('/file.js');
    });
  });
});
