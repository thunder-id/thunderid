// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import GuideCode from '../GuideCode';

describe('GuideCode', () => {
  it('shows the first option until one is picked, and stays on it when nothing tracks the pick', () => {
    render(
      <GuideCode
        options={[
          {label: 'npm', language: 'bash', code: 'npm install @example/react'},
          {label: 'Gradle', code: 'implementation "com.example:sdk"'},
        ]}
        codeValues={{}}
      />,
    );

    expect(screen.getByRole('tab', {name: 'npm'})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByLabelText('npm')).toHaveTextContent('npm install @example/react');

    fireEvent.click(screen.getByRole('tab', {name: 'Gradle'}));

    expect(screen.getByLabelText('npm')).toHaveTextContent('npm install @example/react');
  });
});
