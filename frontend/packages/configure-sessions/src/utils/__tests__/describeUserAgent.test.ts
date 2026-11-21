// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import describeUserAgent from '../describeUserAgent';

describe('describeUserAgent', () => {
  it.each([
    [
      'Chrome on macOS',
      'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
      {browser: 'Chrome', os: 'macOS', mobile: false},
    ],
    [
      'Safari on iOS',
      'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1',
      {browser: 'Safari', os: 'iOS', mobile: true},
    ],
    [
      'Edge on Windows',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0',
      {browser: 'Edge', os: 'Windows', mobile: false},
    ],
    [
      'Firefox on Linux',
      'Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0',
      {browser: 'Firefox', os: 'Linux', mobile: false},
    ],
    [
      'Samsung Internet on Android',
      'Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36',
      {browser: 'Samsung Internet', os: 'Android', mobile: true},
    ],
    [
      'Opera on Windows',
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 OPR/114.0.0.0',
      {browser: 'Opera', os: 'Windows', mobile: false},
    ],
    [
      'Chrome on ChromeOS',
      'Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36',
      {browser: 'Chrome', os: 'ChromeOS', mobile: false},
    ],
    [
      'the Android SDK',
      'ThunderID-Android/1.4.0 (Android 11; Redmi Note 8 Pro)',
      {device: 'Redmi Note 8 Pro', os: 'Android', mobile: true, app: true},
    ],
    [
      'the iOS SDK',
      'ThunderID-iOS/1.4.0 (iOS 15.8; iPhone 7)',
      {device: 'iPhone 7', os: 'iOS', mobile: true, app: true},
    ],
    [
      'a mobile SDK without a device',
      'ThunderID-iOS/1.4.0 (iOS 15.8)',
      {device: undefined, os: 'iOS', mobile: true, app: true},
    ],
  ])('recognizes %s', (_name, userAgent, expected) => {
    expect(describeUserAgent(userAgent)).toEqual(expected);
  });

  it('leaves unrecognized parts undefined', () => {
    expect(describeUserAgent('curl/8.7.1')).toEqual({browser: undefined, os: undefined, mobile: false});
  });

  it('handles a missing User-Agent', () => {
    expect(describeUserAgent(undefined)).toEqual({mobile: false});
    expect(describeUserAgent('')).toEqual({mobile: false});
  });
});
