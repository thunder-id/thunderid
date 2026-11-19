// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import SectionLockNotice from '../SectionLockNotice';

describe('SectionLockNotice', () => {
  it('names the declarative resource file that sets the section', () => {
    render(<SectionLockNotice section="session" />);

    expect(screen.getByRole('alert')).toHaveTextContent(
      "These values come from a declarative resource, so they can't be changed here. To change them, edit server_configs/session.yaml and redeploy.",
    );
    expect(screen.getByText('server_configs/session.yaml').tagName).toBe('CODE');
  });
});
