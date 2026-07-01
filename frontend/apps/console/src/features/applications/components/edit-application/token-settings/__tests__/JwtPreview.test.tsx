/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
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
});
