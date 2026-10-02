// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import ScimPayloadPreview from '../ScimPayloadPreview';

describe('ScimPayloadPreview', () => {
  it('renders the payload as formatted JSON', () => {
    render(<ScimPayloadPreview payload={{userName: '<loginId>'}} />);
    expect(screen.getByText(/"userName": "<loginId>"/)).toBeInTheDocument();
  });

  it('renders an empty object when nothing is mapped', () => {
    render(<ScimPayloadPreview payload={{}} />);
    expect(screen.getByText('{}')).toBeInTheDocument();
  });
});
