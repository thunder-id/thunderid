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

import ThunderIDError from '../ThunderIDError';

describe('ThunderIDError', (): void => {
  it('should create an error with javascript SDK origin', (): void => {
    const message = 'Test error message';
    const code = 'TEST_ERROR';
    const origin = 'javascript';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    expect(error.message).toBe(message);
    expect(error.code).toBe(code);
    expect(error.toString()).toBe(
      '[ThunderIDError]\n⚡ ThunderID - @thunderid/javascript: Test error message\n(code="TEST_ERROR")',
    );
  });

  it('should create an error with react SDK origin', (): void => {
    const message = 'Test error message';
    const code = 'TEST_ERROR';
    const origin = 'react';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    expect(error.message).toBe(message);
    expect(error.code).toBe(code);
    expect(error.toString()).toBe(
      '[ThunderIDError]\n⚡ ThunderID - @thunderid/react: Test error message\n(code="TEST_ERROR")',
    );
  });

  it('should format different SDK origins correctly', (): void => {
    const message = 'Test error message';
    const code = 'TEST_ERROR';
    const origins: string[] = ['react', 'nextjs', 'javascript'];
    const expectedNames: string[] = [
      'ThunderID - @thunderid/react',
      'ThunderID - @thunderid/nextjs',
      'ThunderID - @thunderid/javascript',
    ];

    origins.forEach((origin: string, index: number) => {
      const error: ThunderIDError = new ThunderIDError(message, code, origin);

      expect(error.toString()).toContain(`⚡ ${expectedNames[index]}:`);
    });
  });

  it('should sanitize message if it already contains the SDK prefix', (): void => {
    const message = '⚡ ThunderID - @thunderid/react: Already prefixed message';
    const code = 'TEST_ERROR';
    const origin = 'react';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    expect(error.message).toBe(message);
    expect(error.code).toBe(code);
  });

  it('should have correct name and be instance of Error', (): void => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const origin = 'javascript';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    expect(error.name).toBe('ThunderIDError');
    expect(error).toBeInstanceOf(Error);
    expect(error).toBeInstanceOf(ThunderIDError);
  });

  it('should have a stack trace that includes the error message', () => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const origin = 'javascript';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    expect(error.stack).toBeDefined();
    expect(String(error.stack)).toContain('Test message');
  });

  it('should format toString output correctly with SDK origin', (): void => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const origin = 'react';
    const error: ThunderIDError = new ThunderIDError(message, code, origin);

    const expectedString = '[ThunderIDError]\n⚡ ThunderID - @thunderid/react: Test message\n(code="TEST_ERROR")';

    expect(error.toString()).toBe(expectedString);
  });

  it('should default to the agnostic SDK if no origin is provided', (): void => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const error: ThunderIDError = new ThunderIDError(message, code, '');

    expect(error.origin).toBe('@thunderid/javascript');
  });
});
