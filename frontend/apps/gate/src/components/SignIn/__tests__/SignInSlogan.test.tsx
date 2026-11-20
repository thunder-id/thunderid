// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import SignInSlogan from '../SignInSlogan';

describe('SignInSlogan', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    window.__THUNDERID_RUNTIME_CONFIG__ = undefined;
  });

  it('renders configured slogan items with icons instead of the defaults', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {
        slogan: [{title: 'Cloud Title', description: 'Cloud description.', icon: 'assets/images/cloud.svg'}],
      },
    } as unknown as typeof window.__THUNDERID_RUNTIME_CONFIG__;

    const {container} = render(<SignInSlogan />);

    expect(screen.getByText('Cloud Title')).toBeInTheDocument();
    expect(screen.getByText('Cloud description.')).toBeInTheDocument();
    expect(screen.queryByText('Agent-native Identity')).not.toBeInTheDocument();
    expect(container.querySelector('img[src="/assets/images/cloud.svg"]')).toBeInTheDocument();
  });

  it('renders a string slogan as a single title', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {slogan: 'Sign in to Acme'},
    } as unknown as typeof window.__THUNDERID_RUNTIME_CONFIG__;

    const {container} = render(<SignInSlogan />);

    expect(screen.getByText('Sign in to Acme')).toBeInTheDocument();
    expect(screen.queryByText('Agent-native Identity')).not.toBeInTheDocument();
    expect(container.querySelector('img[src$="icon.svg"]')).not.toBeInTheDocument();
  });

  it('renders an object slogan as a single item', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {slogan: {title: 'Single Title', description: 'Single description.', icon: 'assets/images/single.svg'}},
    } as unknown as typeof window.__THUNDERID_RUNTIME_CONFIG__;

    const {container} = render(<SignInSlogan />);

    expect(screen.getByText('Single Title')).toBeInTheDocument();
    expect(screen.getByText('Single description.')).toBeInTheDocument();
    expect(screen.queryByText('Agent-native Identity')).not.toBeInTheDocument();
    expect(container.querySelector('img[src="/assets/images/single.svg"]')).toBeInTheDocument();
  });

  it('falls back to the default items when the slogan is a blank string', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {slogan: '   '},
    } as unknown as typeof window.__THUNDERID_RUNTIME_CONFIG__;

    render(<SignInSlogan />);

    expect(screen.getByText('Agent-native Identity')).toBeInTheDocument();
  });

  it('falls back to the default items when the configured list is empty', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = {
      brand: {slogan: []},
    } as unknown as typeof window.__THUNDERID_RUNTIME_CONFIG__;

    render(<SignInSlogan />);

    expect(screen.getByText('Agent-native Identity')).toBeInTheDocument();
  });

  it('renders without crashing', () => {
    const {container} = render(<SignInSlogan />);
    expect(container).toBeInTheDocument();
  });

  it('renders all slogan items', () => {
    render(<SignInSlogan />);
    expect(screen.getByText('Agent-native Identity')).toBeInTheDocument();
    expect(screen.getByText('Post-quantum-safe by Design')).toBeInTheDocument();
    expect(screen.getByText('Lightweight Runtime with GitOps Support')).toBeInTheDocument();
  });

  it('renders item descriptions', () => {
    render(<SignInSlogan />);
    expect(screen.getByText(/Engineered with native Agent ID/)).toBeInTheDocument();
    expect(screen.getByText(/Built on a post-quantum cryptographic foundation/)).toBeInTheDocument();
    expect(screen.getByText(/Cloud-native, API-first runtime/)).toBeInTheDocument();
  });

  it('renders with default logos', () => {
    const {rerender} = render(<SignInSlogan />);
    rerender(<SignInSlogan />);

    const logo = screen.getByAltText('Logo (Light)');

    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute('src', expect.stringContaining('/assets/images/logo.svg'));
    expect(logo).toHaveStyle({height: '50px'});
  });
});
