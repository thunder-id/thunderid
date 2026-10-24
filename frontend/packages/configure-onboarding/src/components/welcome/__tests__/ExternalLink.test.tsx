// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import {MCP_INSPECTOR_URL} from '../../../constants/sample-urls';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    ExternalLink: () => <span data-testid="icon-external-link" />,
  };
});

import ExternalLink from '../ExternalLink';

describe('ExternalLink', () => {
  it('renders an anchor with the given href', () => {
    render(<ExternalLink href={MCP_INSPECTOR_URL}>Open Inspector</ExternalLink>);
    const link = screen.getByRole('link', {name: /Open Inspector/});
    expect(link).toHaveAttribute('href', 'http://localhost:6274');
  });

  it('opens in a new tab with noopener noreferrer', () => {
    render(<ExternalLink href={MCP_INSPECTOR_URL}>Link</ExternalLink>);
    const link = screen.getByRole('link', {name: /Link/});
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('renders the external link icon', () => {
    render(<ExternalLink href={MCP_INSPECTOR_URL}>Link</ExternalLink>);
    expect(screen.getByTestId('icon-external-link')).toBeInTheDocument();
  });

  it('renders children text', () => {
    render(<ExternalLink href={MCP_INSPECTOR_URL}>Click here</ExternalLink>);
    expect(screen.getByText('Click here')).toBeInTheDocument();
  });

  it('renders without children', () => {
    render(<ExternalLink href={MCP_INSPECTOR_URL} />);
    expect(screen.getByRole('link')).toBeInTheDocument();
  });
});
