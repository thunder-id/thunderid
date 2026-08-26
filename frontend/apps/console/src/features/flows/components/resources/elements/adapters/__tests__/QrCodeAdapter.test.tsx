// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import QrCodeAdapter, {type QrCodeElement} from '../QrCodeAdapter';

describe('QrCodeAdapter', () => {
  const createResource = (source?: string): QrCodeElement =>
    ({
      id: 'qr-1',
      ...(source !== undefined ? {source} : {}),
    }) as unknown as QrCodeElement;

  it('should render the bound source key on the canvas', () => {
    render(<QrCodeAdapter resource={createResource('openid4vpWalletUri')} />);

    expect(screen.getByText('{{openid4vpWalletUri}}')).toBeInTheDocument();
  });

  it('should indicate when no source is bound', () => {
    render(<QrCodeAdapter resource={createResource()} />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });

  it('should indicate when the source is an empty string', () => {
    render(<QrCodeAdapter resource={createResource('')} />);

    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });

  it('should render the placeholder when resource is undefined', () => {
    render(<QrCodeAdapter />);

    expect(screen.getByRole('img', {name: 'QR Code'})).toBeInTheDocument();
    expect(screen.getByText('No source bound')).toBeInTheDocument();
  });

  it('should render a stable symbol across renders', () => {
    const {container: first} = render(<QrCodeAdapter resource={createResource('openid4vpWalletUri')} />);
    const {container: second} = render(<QrCodeAdapter resource={createResource('openid4vpWalletUri')} />);

    const firstSvg: SVGSVGElement | null = first.querySelector('svg');
    const secondSvg: SVGSVGElement | null = second.querySelector('svg');

    expect(firstSvg).not.toBeNull();
    expect(secondSvg).not.toBeNull();
    expect(firstSvg!.innerHTML).toBe(secondSvg!.innerHTML);
  });
});
