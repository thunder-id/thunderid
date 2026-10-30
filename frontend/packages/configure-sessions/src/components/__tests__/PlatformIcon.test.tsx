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
        <PlatformIcon name="macOS" kind="os" />
      </>,
    );

    expect(container.querySelectorAll('svg:not(.lucide) > path')).toHaveLength(2);
  });

  it('falls back to a generic icon for one without a logo', () => {
    const {container} = render(
      <>
        <PlatformIcon name="Edge" kind="browser" />
        <PlatformIcon name="Safari" kind="browser" />
        <PlatformIcon kind="os" />
      </>,
    );

    expect(container.querySelectorAll('svg.lucide')).toHaveLength(3);
  });
});
