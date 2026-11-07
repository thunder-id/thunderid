// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect} from 'vitest';
import SignInCallbackLoading from '../SignInCallbackLoading';

describe('SignInCallbackLoading', () => {
  it('shows a spinner that announces sign-in is in progress', () => {
    render(<SignInCallbackLoading />);

    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.getByRole('progressbar', {name: 'Signing you in'})).toBeInTheDocument();
  });

  it('shows only the spinner, with no visible text or image', () => {
    const {container} = render(<SignInCallbackLoading />);

    expect(screen.getByRole('status')).toHaveTextContent('');
    expect(container.querySelector('img')).not.toBeInTheDocument();
  });
});
