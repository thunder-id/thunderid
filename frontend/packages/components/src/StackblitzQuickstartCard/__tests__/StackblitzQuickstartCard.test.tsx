// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import StackblitzQuickstartCard from '../StackblitzQuickstartCard';

describe('StackblitzQuickstartCard', () => {
  it('renders the title and links to the given StackBlitz URL in a new tab', () => {
    render(
      <StackblitzQuickstartCard
        url="https://stackblitz.com/fork/github/thunder-id/samples/tree/main/quickstarts/react"
        heading="Try the live quickstart"
        subheading="Run react-quickstart in StackBlitz"
        ctaLabel="Open on StackBlitz"
      />,
    );

    const link = screen.getByRole('link');
    expect(link).toHaveAttribute(
      'href',
      'https://stackblitz.com/fork/github/thunder-id/samples/tree/main/quickstarts/react',
    );
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    expect(screen.getByText('Try the live quickstart')).toBeInTheDocument();
    expect(screen.getByText('Run react-quickstart in StackBlitz')).toBeInTheDocument();
    expect(screen.getByText('Open on StackBlitz')).toBeInTheDocument();
  });
});
