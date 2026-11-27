// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import PlatformIcon from '../PlatformIcon';

describe('PlatformIcon', () => {
  it('draws the logo of a known browser or operating system', () => {
    const {container} = render(
      <>
        <PlatformIcon name="Chrome" kind="browser" />
        <PlatformIcon name="Firefox" kind="browser" />
        <PlatformIcon name="Opera" kind="browser" />
        <PlatformIcon name="Safari" kind="browser" />
        <PlatformIcon name="macOS" kind="os" />
        <PlatformIcon name="iOS" kind="os" />
        <PlatformIcon name="Windows" kind="os" />
        <PlatformIcon name="Android" kind="os" />
        <PlatformIcon name="Linux" kind="os" />
        <PlatformIcon name="ChromeOS" kind="os" />
      </>,
    );

    expect(container.querySelectorAll('svg:not(.lucide) > path')).toHaveLength(10);
    expect(container.querySelectorAll('svg.lucide')).toHaveLength(0);
  });

  it('falls back to a generic icon for one without a logo', () => {
    const {container} = render(
      <>
        <PlatformIcon name="Edge" kind="browser" />
        <PlatformIcon name="Samsung Internet" kind="browser" />
        <PlatformIcon kind="os" />
      </>,
    );

    expect(container.querySelectorAll('svg.lucide')).toHaveLength(3);
  });
});
