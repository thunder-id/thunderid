// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import ScimPayloadPreview from '../ScimPayloadPreview';

describe('ScimPayloadPreview', () => {
  it('renders the payload as formatted JSON with syntax highlighting', () => {
    render(
      <ScimPayloadPreview
        payload={{
          userName: '<loginId>',
          active: true,
          count: 2,
          manager: null,
        }}
      />,
    );

    const preview = screen.getByTestId('scim-payload-preview');
    expect(preview).toHaveTextContent('"userName": "<loginId>"');
    expect(screen.getByText('"userName"')).toHaveStyle({color: '#9cdcfe'});
    expect(screen.getByText('"<loginId>"')).toHaveStyle({color: '#ce9178'});
    expect(screen.getByText('true')).toHaveStyle({color: '#569cd6'});
    expect(screen.getByText('2')).toHaveStyle({color: '#b5cea8'});
    expect(screen.getByText('null')).toHaveStyle({color: '#569cd6'});
  });

  it('renders an empty object when nothing is mapped', () => {
    render(<ScimPayloadPreview payload={{}} />);
    expect(screen.getByText('{}')).toBeInTheDocument();
  });
});
