// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, waitFor} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import DocumentTitle from '../DocumentTitle';

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    config: {brand: {product_name: 'ThunderID'}},
  }),
}));

describe('DocumentTitle', () => {
  it('renders the page title followed by the product name', async () => {
    render(<DocumentTitle title="Users" />);

    await waitFor(() => expect(document.title).toBe('Users | ThunderID'));
  });

  it('renders only the product name when no title is given', async () => {
    render(<DocumentTitle />);

    await waitFor(() => expect(document.title).toBe('ThunderID'));
  });

  it('renders only the product name when the title is empty', async () => {
    render(<DocumentTitle title="" />);

    await waitFor(() => expect(document.title).toBe('ThunderID'));
  });

  it('updates the title when the prop changes', async () => {
    const {rerender} = render(<DocumentTitle title="Group" />);
    await waitFor(() => expect(document.title).toBe('Group | ThunderID'));

    rerender(<DocumentTitle title="Admins" />);

    await waitFor(() => expect(document.title).toBe('Admins | ThunderID'));
  });
});
