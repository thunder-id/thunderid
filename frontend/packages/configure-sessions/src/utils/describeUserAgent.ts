// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The browser, operating system, device model and kind of device a User-Agent names. A part the string
 * doesn't reveal is left undefined.
 */
export interface UserAgentDescription {
  browser?: string;
  device?: string;
  os?: string;
  mobile: boolean;
  /** Whether a mobile SDK sent it, so the session is a native app's and has no browser. */
  app?: boolean;
}

// The User-Agent the mobile SDKs send: `ThunderID-Android/1.4.0 (Android 14; Google Pixel 8)`, or the
// iOS equivalent. The Flutter SDK bridges to those, so its requests carry the same header. The
// first word of the comment is the operating system, the part after `;` the device.
const NATIVE_SDK = /\bThunderID-(?:Android|iOS)\/\S*\s*\((\w+)[^;)]*(?:;\s*([^)]+))?\)/;

// Checked in order: several browsers also carry the tokens of the ones they derive from, so Edge,
// Opera and Samsung Internet come before Chrome, and Chrome before Safari.
const BROWSERS: [RegExp, string][] = [
  [/\bEdg(?:e|A|iOS)?\//, 'Edge'],
  [/\b(?:OPR|Opera)\//, 'Opera'],
  [/\bSamsungBrowser\//, 'Samsung Internet'],
  [/\b(?:Firefox|FxiOS)\//, 'Firefox'],
  [/\b(?:Chrome|CriOS)\//, 'Chrome'],
  [/\bVersion\/[\d.]+.*\bSafari\//, 'Safari'],
];

const OPERATING_SYSTEMS: [RegExp, string][] = [
  [/\bWindows\b/, 'Windows'],
  [/\b(?:iPhone|iPad|iPod)\b/, 'iOS'],
  [/\bAndroid\b/, 'Android'],
  [/\bCrOS\b/, 'ChromeOS'],
  [/\bMac OS X\b|\bMacintosh\b/, 'macOS'],
  [/\bLinux\b/, 'Linux'],
];

function firstMatch(userAgent: string, patterns: [RegExp, string][]): string | undefined {
  return patterns.find(([pattern]) => pattern.test(userAgent))?.[1];
}

/**
 * Reads the browser and operating system from a User-Agent, or the device from a mobile SDK's, for
 * showing a session as, for example, "Chrome on macOS". It recognizes the common browsers only, and is for display, never for decisions.
 *
 * @param userAgent - The raw User-Agent string
 * @returns The parts of the User-Agent that could be recognized
 */
export default function describeUserAgent(userAgent: string | undefined): UserAgentDescription {
  if (!userAgent) {
    return {mobile: false};
  }
  const native = NATIVE_SDK.exec(userAgent);
  if (native) {
    return {device: native[2]?.trim() || undefined, os: native[1], mobile: true, app: true};
  }
  return {
    browser: firstMatch(userAgent, BROWSERS),
    os: firstMatch(userAgent, OPERATING_SYSTEMS),
    mobile: /\b(?:Mobi|iPhone|iPad|iPod|Android)\b/.test(userAgent),
  };
}
