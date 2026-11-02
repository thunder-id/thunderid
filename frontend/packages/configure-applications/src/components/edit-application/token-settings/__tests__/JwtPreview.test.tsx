// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import JwtPreview from '../JwtPreview';

vi.mock('@monaco-editor/react', () => ({
  default: ({value}: {value: string}) => <pre data-testid="monaco-editor">{value}</pre>,
}));

describe('JwtPreview', () => {
  it('renders the JWT logo SVG element', () => {
    const {container} = render(<JwtPreview payload={{sub: 'user-123'}} />);

    expect(container.querySelector('svg')).toBeInTheDocument();
  });

  it('renders the payload as JSON in the editor', () => {
    const payload = {sub: 'user-123', iss: 'https://example.com'};
    render(<JwtPreview payload={payload} />);

    const editor = screen.getByTestId('monaco-editor');
    const content = editor.textContent ?? '';

    expect(content).toContain('"sub"');
    expect(content).toContain('"user-123"');
    expect(content).toContain('"iss"');
    expect(content).toContain('"https://example.com"');
  });

  it('renders without errors when defaultClaims prop is provided', () => {
    expect(() =>
      render(<JwtPreview payload={{sub: 'user-123', iss: 'https://example.com'}} defaultClaims={['sub', 'iss']} />),
    ).not.toThrow();
  });

  it('renders an empty JSON object when payload is empty', () => {
    render(<JwtPreview payload={{}} />);

    const editor = screen.getByTestId('monaco-editor');

    expect(editor.textContent).toContain('{}');
  });

  it('renders header section when header prop is provided', () => {
    const header = {alg: 'RS256', typ: 'JWT'};
    render(<JwtPreview payload={{sub: 'user-123'}} header={header} />);

    const editors = screen.getAllByTestId('monaco-editor');
    expect(editors).toHaveLength(2);
    expect(editors[0].textContent).toContain('"alg"');
    expect(editors[0].textContent).toContain('"RS256"');
  });

  it('renders Header and Payload labels when header is provided', () => {
    const header = {alg: 'RS256', typ: 'JWT'};
    render(<JwtPreview payload={{sub: 'user-123'}} header={header} />);

    expect(screen.getByText('Decoded Header')).toBeInTheDocument();
    expect(screen.getByText('Decoded Payload')).toBeInTheDocument();
  });

  it('does not render Header label when header is not provided', () => {
    render(<JwtPreview payload={{sub: 'user-123'}} />);

    expect(screen.queryByText('Decoded Header')).not.toBeInTheDocument();
    expect(screen.queryByText('Decoded Payload')).not.toBeInTheDocument();
  });

  describe('default claim styling', () => {
    afterEach(() => {
      document.querySelector('meta[property="csp-nonce"]')?.remove();
    });

    it("tags the default-claim <style> tag with the page's csp nonce", () => {
      const meta = document.createElement('meta');
      meta.setAttribute('property', 'csp-nonce');
      meta.setAttribute('content', 'abc123');
      document.head.appendChild(meta);

      const {container} = render(<JwtPreview payload={{sub: 'user-123'}} />);

      expect(container.querySelector('style')).toHaveAttribute('nonce', 'abc123');
    });

    it('renders the <style> tag without a nonce attribute when none is available', () => {
      const {container} = render(<JwtPreview payload={{sub: 'user-123'}} />);

      expect(container.querySelector('style')).not.toHaveAttribute('nonce');
    });
  });
});
