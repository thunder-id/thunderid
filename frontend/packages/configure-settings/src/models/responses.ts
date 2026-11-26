// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export interface RegexOrigin {
  regex: string;
}

/**
 * A single allowed origin: a literal string (including the `"null"` literal) or a regex entry.
 */
export type AllowedOrigin = string | RegexOrigin;

export interface CorsValue {
  allowedOrigins: AllowedOrigin[];
}

/**
 * A server-config section returned as its layers: `readOnly` (declarative), `writable`
 * (runtime-mutable), `default`, and `merged` (effective).
 */
export interface ServerConfigLayers<T> {
  /** Declarative baseline, not editable at runtime */
  readOnly: T;
  /** Runtime-mutable layer edited through the console */
  writable: T;
  /** The value a field takes when the section leaves it unset, or null when the section has none */
  default?: T | null;
  /** Effective union of the read-only and writable layers, laid over the default */
  merged: T;
}

export type CorsConfigResponse = ServerConfigLayers<CorsValue>;

/**
 * A map of CSP directive name (for example `img-src`) to its list of sources.
 */
export type CspDirectives = Record<string, string[]>;

/**
 * CSP directives that apply only to requests whose path starts with `location`.
 */
export interface CspPathPolicy {
  location: string;
  directives: CspDirectives;
}

export interface CspValue {
  /** Unset means report-only, so a policy is never enforced until explicitly switched. */
  reportOnly?: boolean;
  reportUri?: string;
  directives?: CspDirectives;
  paths?: CspPathPolicy[];
}

export type CspConfigResponse = ServerConfigLayers<CspValue>;
