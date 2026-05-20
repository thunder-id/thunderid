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
 * Log levels enum
 */
export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

/**
 * Logger configuration interface
 */
export interface LoggerConfig {
  /** Custom log formatter function */
  formatter?: (level: LogLevel, message: string, ...args: any[]) => void;
  /** Minimum log level to output */
  level: LogLevel;
  /** Custom prefix for all log messages */
  prefix?: string;
  /** Whether to include log level in output */
  showLevel?: boolean;
  /** Whether to include timestamps */
  timestamps?: boolean;
}

const PREFIX = '⚡ ThunderID';

/**
 * Default logger configuration
 */
const DEFAULT_CONFIG: LoggerConfig = {
  level: 'info',
  prefix: `${PREFIX}`,
  showLevel: true,
  timestamps: true,
};

/**
 * Environment detection utilities
 */
const isBrowser = (): boolean =>
  /* @ts-ignore */
  typeof window !== 'undefined' && typeof window.document !== 'undefined';
const isNode = (): boolean =>
  /* @ts-ignore */
  typeof process !== 'undefined' && process.versions && process.versions.node;
/**
 * Color codes for terminal output (Node.js)
 */
const COLORS: {
  blue: string;
  bright: string;
  cyan: string;
  dim: string;
  gray: string;
  green: string;
  magenta: string;
  red: string;
  reset: string;
  white: string;
  yellow: string;
} = {
  blue: '\x1b[34m',
  bright: '\x1b[1m',
  cyan: '\x1b[36m',
  dim: '\x1b[2m',
  gray: '\x1b[90m',
  green: '\x1b[32m',
  magenta: '\x1b[35m',
  red: '\x1b[31m',
  reset: '\x1b[0m',
  white: '\x1b[37m',
  yellow: '\x1b[33m',
};

/**
 * Browser console styling
 */
const BROWSER_STYLES: {
  debug: string;
  error: string;
  info: string;
  prefix: string;
  timestamp: string;
  warn: string;
} = {
  debug: 'color: #6b7280; font-weight: normal;',
  error: 'color: #dc2626; font-weight: bold;',
  info: 'color: #2563eb; font-weight: bold;',
  prefix: 'color: #7c3aed; font-weight: bold;',
  timestamp: 'color: #6b7280; font-size: 0.9em;',
  warn: 'color: #d97706; font-weight: bold;',
};

const LOG_LEVEL_ORDER: Record<LogLevel, number> = {
  debug: 0,
  error: 3,
  info: 1,
  warn: 2,
};

/**
 * Universal logger class that works in both browser and Node.js environments
 */
class Logger {
  private config: LoggerConfig;

  constructor(config: Partial<LoggerConfig> = {}) {
    this.config = {...DEFAULT_CONFIG, ...config};
  }

  /**
   * Update logger configuration
   */
  configure(config: Partial<LoggerConfig>): void {
    this.config = {...this.config, ...config};
  }

  /**
   * Get current configuration
   */
  getConfig(): LoggerConfig {
    return {...this.config};
  }

  /**
   * Check if a log level should be output
   */
  private shouldLog(level: LogLevel): boolean {
    return LOG_LEVEL_ORDER[level] >= LOG_LEVEL_ORDER[this.config.level];
  }

  /**
   * Get timestamp string
   */
  private static getTimestamp(): string {
    return new Date().toISOString();
  }

  /**
   * Get log level string
   */
  private static getLevelString(level: LogLevel): string {
    switch (level) {
      case 'debug':
        return 'DEBUG';
      case 'info':
        return 'INFO';
      case 'warn':
        return 'WARN';
      case 'error':
        return 'ERROR';
      default:
        return 'UNKNOWN';
    }
  }

  /**
   * Format message for Node.js terminal
   */
  private formatForNode(level: LogLevel, message: string): string {
    const parts: string[] = [];

    // Add timestamp
    if (this.config.timestamps) {
      parts.push(`${COLORS.gray}[${Logger.getTimestamp()}]${COLORS.reset}`);
    }

    // Add prefix
    if (this.config.prefix) {
      parts.push(`${COLORS.magenta}${this.config.prefix}${COLORS.reset}`);
    }

    // Add log level
    if (this.config.showLevel) {
      const levelStr: string = Logger.getLevelString(level);
      let coloredLevel: string;

      switch (level) {
        case 'debug':
          coloredLevel = `${COLORS.gray}[${levelStr}]${COLORS.reset}`;
          break;
        case 'info':
          coloredLevel = `${COLORS.blue}[${levelStr}]${COLORS.reset}`;
          break;
        case 'warn':
          coloredLevel = `${COLORS.yellow}[${levelStr}]${COLORS.reset}`;
          break;
        case 'error':
          coloredLevel = `${COLORS.red}[${levelStr}]${COLORS.reset}`;
          break;
        default:
          coloredLevel = `[${levelStr}]`;
      }
      parts.push(coloredLevel);
    }

    // Add message
    parts.push(message);

    return parts.join(' ');
  }

  /**
   * Log message using appropriate method
   */
  private logMessage(level: LogLevel, message: string, ...args: any[]): void {
    if (!this.shouldLog(level)) {
      return;
    }

    // Use custom formatter if provided
    if (this.config.formatter) {
      this.config.formatter(level, message, ...args);
      return;
    }

    if (isBrowser()) {
      this.logToBrowser(level, message, ...args);
    } else if (isNode()) {
      this.logToNode(level, message, ...args);
    } else {
      // Fallback for other environments
      // eslint-disable-next-line no-console
      console.log(message, ...args);
    }
  }

  /**
   * Log to browser console with styling
   */
  private logToBrowser(level: LogLevel, message: string, ...args: any[]): void {
    const parts: string[] = [];
    const styles: string[] = [];

    // Add timestamp
    if (this.config.timestamps) {
      parts.push(`%c[${Logger.getTimestamp()}]`);
      styles.push(BROWSER_STYLES.timestamp);
    }

    // Add prefix
    if (this.config.prefix) {
      parts.push(`%c${this.config.prefix}`);
      styles.push(BROWSER_STYLES.prefix);
    }

    // Add log level and message
    if (this.config.showLevel) {
      const levelStr: string = Logger.getLevelString(level);
      parts.push(`%c[${levelStr}]`);

      switch (level) {
        case 'debug':
          styles.push(BROWSER_STYLES.debug);
          break;
        case 'info':
          styles.push(BROWSER_STYLES.info);
          break;
        case 'warn':
          styles.push(BROWSER_STYLES.warn);
          break;
        case 'error':
          styles.push(BROWSER_STYLES.error);
          break;
        default:
          styles.push('');
      }
    }

    parts.push(`%c${message}`);
    styles.push('color: inherit; font-weight: normal;');

    const formattedMessage: string = parts.join(' ');

    // Use appropriate console method
    switch (level) {
      case 'debug':
        // eslint-disable-next-line no-console
        console.debug(formattedMessage, ...styles, ...args);
        break;
      case 'info':
        // eslint-disable-next-line no-console
        console.info(formattedMessage, ...styles, ...args);
        break;
      case 'warn':
        // eslint-disable-next-line no-console
        console.warn(formattedMessage, ...styles, ...args);
        break;
      case 'error':
        // eslint-disable-next-line no-console
        console.error(formattedMessage, ...styles, ...args);
        break;
      default:
        // eslint-disable-next-line no-console
        console.log(formattedMessage, ...styles, ...args);
    }
  }

  /**
   * Log to Node.js console
   */
  private logToNode(level: LogLevel, message: string, ...args: any[]): void {
    const formattedMessage: string = this.formatForNode(level, message);

    // Use appropriate console method
    switch (level) {
      case 'debug':
        // eslint-disable-next-line no-console
        console.debug(formattedMessage, ...args);
        break;
      case 'info':
        // eslint-disable-next-line no-console
        console.info(formattedMessage, ...args);
        break;
      case 'warn':
        // eslint-disable-next-line no-console
        console.warn(formattedMessage, ...args);
        break;
      case 'error':
        // eslint-disable-next-line no-console
        console.error(formattedMessage, ...args);
        break;
      default:
        // eslint-disable-next-line no-console
        console.log(formattedMessage, ...args);
    }
  }

  /**
   * Log debug message
   */
  debug(message: string, ...args: any[]): void {
    this.logMessage('debug', message, ...args);
  }

  /**
   * Log info message
   */
  info(message: string, ...args: any[]): void {
    this.logMessage('info', message, ...args);
  }

  /**
   * Log warning message
   */
  warn(message: string, ...args: any[]): void {
    this.logMessage('warn', message, ...args);
  }

  /**
   * Log error message
   */
  error(message: string, ...args: any[]): void {
    this.logMessage('error', message, ...args);
  }

  /**
   * Create a child logger with additional prefix
   */
  child(prefix: string): Logger {
    const childPrefix: string = this.config.prefix ? `${this.config.prefix} - ${prefix}` : prefix;
    return new Logger({
      ...this.config,
      prefix: childPrefix,
    });
  }

  /**
   * Set log level
   */
  setLevel(level: LogLevel): void {
    this.config.level = level;
  }

  /**
   * Get current log level
   */
  getLevel(): LogLevel {
    return this.config.level;
  }
}

/**
 * Default logger instance
 */
const logger: Logger = new Logger();

/**
 * Create a new logger instance with custom configuration
 */
export const createLogger = (config?: Partial<LoggerConfig>): Logger => new Logger(config);

/**
 * Default export - global logger instance
 */
export default logger;

/**
 * Named exports for convenience
 */
export const debug = (message: string, ...args: any[]): void => logger.debug(message, ...args);
export const info = (message: string, ...args: any[]): void => logger.info(message, ...args);
export const warn = (message: string, ...args: any[]): void => logger.warn(message, ...args);
export const error = (message: string, ...args: any[]): void => logger.error(message, ...args);

/**
 * Configure the default logger
 */
export const configure = (config: Partial<LoggerConfig>): void => logger.configure(config);

/**
 * Create component-specific loggers
 */
export const createComponentLogger = (component: string): Logger => logger.child(component);

/**
 * Create package-specific logger
 */
export const createPackageLogger = (packageName: string): Logger =>
  createLogger({
    level: 'info',
    prefix: `${PREFIX} - ${packageName}`,
    showLevel: true,
    timestamps: true,
  });

/**
 * Create package component logger (package + component)
 */
export const createPackageComponentLogger = (packageName: string, component: string): Logger => {
  const packageLogger: Logger = createPackageLogger(packageName);
  return packageLogger.child(component);
};
