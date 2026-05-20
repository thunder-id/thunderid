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
import ThunderIDRuntimeError from '../ThunderIDRuntimeError';

describe('ThunderIDRuntimeError', (): void => {
  it('should create a runtime error with details', (): void => {
    const message = 'Configuration Error';
    const code = 'CONFIG_ERROR';
    const origin = 'react';
    const details: {invalidField: string; value: null} = {invalidField: 'redirectUri', value: null};
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin, details);

    expect(error.message).toBe(message);
    expect(error.code).toBe(code);
    expect(error.details).toEqual(details);
    expect(error.toString()).toContain(
      '[ThunderIDRuntimeError] (code="CONFIG_ERROR")\nDetails: {\n  "invalidField": "redirectUri",\n  "value": null\n}\nMessage: Configuration Error',
    );
  });

  it('should create a runtime error without details', (): void => {
    const message = 'Unknown Runtime Error';
    const code = 'RUNTIME_ERROR';
    const origin = 'javascript';
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin);

    expect(error.message).toBe(message);
    expect(error.details).toBeUndefined();
    expect(error.toString()).toContain(
      '[ThunderIDRuntimeError] (code="RUNTIME_ERROR")\nMessage: Unknown Runtime Error',
    );
  });

  it('should have correct name and be instance of Error, ThunderIDError, and ThunderIDRuntimeError', (): void => {
    const message = 'Test Error';
    const code = 'TEST_ERROR';
    const origin = 'react';
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin);

    expect(error.name).toBe('ThunderIDRuntimeError');
    expect(error).toBeInstanceOf(Error);
    expect(error).toBeInstanceOf(ThunderIDError);
    expect(error).toBeInstanceOf(ThunderIDRuntimeError);
  });

  it('should format toString with details when available', (): void => {
    const message = 'Validation Error';
    const code = 'VALIDATION_ERROR';
    const origin = 'react';
    const details: {field: string; reason: string} = {field: 'email', reason: 'invalid_input'};
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin, details);

    const expected: string =
      '[ThunderIDRuntimeError] (code="VALIDATION_ERROR")\n' +
      'Details: {\n  "field": "email",\n  "reason": "invalid_input"\n}\n' +
      'Message: Validation Error';

    expect(error.toString()).toBe(expected);
  });

  it('should format toString without details when not available', (): void => {
    const message = 'Test Error';
    const code = 'TEST_ERROR';
    const origin = 'react';
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin);

    const expected = '[ThunderIDRuntimeError] (code="TEST_ERROR")\nMessage: Test Error';

    expect(error.toString()).toBe(expected);
  });

  it('should default to the agnostic SDK if no origin is provided', (): void => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const error: ThunderIDError = new ThunderIDRuntimeError(message, code, '');

    expect(error.origin).toBe('@thunderid/javascript');
  });

  it('should have a stack trace that includes the error message', () => {
    const message = 'Test message';
    const code = 'TEST_ERROR';
    const origin = 'javascript';
    const error: ThunderIDRuntimeError = new ThunderIDRuntimeError(message, code, origin);

    expect(error.stack).toBeDefined();
    expect(String(error.stack)).toContain('Test message');
  });
});
