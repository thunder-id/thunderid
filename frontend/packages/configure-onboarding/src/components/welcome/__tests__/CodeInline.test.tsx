// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import CodeInline from '../CodeInline';

describe('CodeInline', () => {
  it('renders children', () => {
    render(<CodeInline>my-token</CodeInline>);
    expect(screen.getByText('my-token')).toBeInTheDocument();
  });

  it('renders as a code element', () => {
    const {container} = render(<CodeInline>value</CodeInline>);
    expect(container.querySelector('code')).toBeInTheDocument();
  });

  it('renders without children', () => {
    const {container} = render(<CodeInline />);
    expect(container).toBeInTheDocument();
  });
});
