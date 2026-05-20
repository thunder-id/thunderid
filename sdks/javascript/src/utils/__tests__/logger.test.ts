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

import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import logger, {createLogger, createComponentLogger} from '../logger';

type Logger = ReturnType<typeof createLogger>;

describe('Logger', () => {
  let infoSpy: ReturnType<typeof vi.spyOn>;
  let warnSpy: ReturnType<typeof vi.spyOn>;
  let errorSpy: ReturnType<typeof vi.spyOn>;
  let debugSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    // Reset logger state if it is a singleton
    if (logger?.setLevel) {
      logger.setLevel('info' as any);
    }

    // Spy on console methods
    vi.spyOn(console, 'log').mockImplementation(() => {});
    infoSpy = vi.spyOn(console, 'info').mockImplementation(() => {});
    warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
    errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    debugSpy = vi.spyOn(console, 'debug').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Basic logging', () => {
    it('should log info messages', () => {
      logger.info('Test info message');
      expect(infoSpy).toHaveBeenCalled();
    });

    it('should log warning messages', () => {
      logger.warn('Test warning message');
      expect(warnSpy).toHaveBeenCalled();
    });

    it('should log error messages', () => {
      logger.error('Test error message');
      expect(errorSpy).toHaveBeenCalled();
    });

    it('should log debug messages when level is debug', () => {
      logger.setLevel('debug' as any);
      // eslint-disable-next-line testing-library/no-debugging-utils
      logger.debug('Test debug message');
      expect(debugSpy).toHaveBeenCalled();
    });

    it('should not log debug messages when level is info', () => {
      logger.setLevel('info' as any);
      // eslint-disable-next-line testing-library/no-debugging-utils
      logger.debug('Test debug message');
      expect(debugSpy).not.toHaveBeenCalled();
    });
  });

  describe('Log levels', () => {
    it('should respect log level filtering', () => {
      logger.setLevel('warn' as any);

      // eslint-disable-next-line testing-library/no-debugging-utils
      logger.debug('Debug message');
      logger.info('Info message');
      logger.warn('Warning message');
      logger.error('Error message');

      expect(debugSpy).not.toHaveBeenCalled();
      expect(infoSpy).not.toHaveBeenCalled();
      expect(warnSpy).toHaveBeenCalled();
      expect(errorSpy).toHaveBeenCalled();
    });

    it('should silence all logs when level is silent', () => {
      // cast to any if the type union does not include "silent"
      logger.setLevel('silent' as any);

      // eslint-disable-next-line testing-library/no-debugging-utils
      logger.debug('Debug message');
      logger.info('Info message');
      logger.warn('Warning message');
      logger.error('Error message');

      expect(debugSpy).not.toHaveBeenCalled();
      expect(infoSpy).not.toHaveBeenCalled();
      expect(warnSpy).not.toHaveBeenCalled();
      expect(errorSpy).not.toHaveBeenCalled();
    });
  });

  describe('Custom loggers', () => {
    it('should create logger with custom configuration', () => {
      const customLogger: Logger = createLogger({
        level: 'debug' as any,
        prefix: 'Custom',
        showLevel: false,
        timestamps: false,
      });

      expect(customLogger.getLevel()).toBe('debug');
      expect(customLogger.getConfig().prefix).toBe('Custom');
      expect(customLogger.getConfig().timestamps).toBe(false);
      expect(customLogger.getConfig().showLevel).toBe(false);
    });

    it('should create component logger with nested prefix', () => {
      const componentLogger: Logger = createComponentLogger('Authentication');
      componentLogger.info('Test message');
      expect(infoSpy).toHaveBeenCalled();
    });

    it('should create child logger', () => {
      const parentLogger: Logger = createLogger({prefix: 'Parent'});
      const childLogger: Logger = parentLogger.child('Child');
      expect(childLogger.getConfig().prefix).toBe('Parent - Child');
    });
  });

  describe('Configuration', () => {
    it('should update configuration', () => {
      const testLogger: Logger = createLogger({level: 'info' as any});

      testLogger.configure({
        level: 'debug' as any,
        prefix: 'Updated',
      });

      expect(testLogger.getLevel()).toBe('debug');
      expect(testLogger.getConfig().prefix).toBe('Updated');
    });
  });

  describe('Custom formatter', () => {
    it('should use custom formatter when provided', () => {
      const mockFormatter: ReturnType<typeof vi.fn> = vi.fn();
      const customLogger: Logger = createLogger({formatter: mockFormatter});

      customLogger.info('Test message', {data: 'test'});

      expect(mockFormatter).toHaveBeenCalledWith('info', 'Test message', {data: 'test'});
      // When a custom formatter is provided, the logger should defer to it
      expect(infoSpy).not.toHaveBeenCalled();
    });
  });
});
