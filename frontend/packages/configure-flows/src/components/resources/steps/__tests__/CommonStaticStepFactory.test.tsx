// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import {StaticStepTypes} from '../../../../models/steps';
import {CommonStaticStepFactory} from '../CommonStaticStepFactory';

// Mock the Start component
vi.mock('../start/Start', () => ({
  default: () => <div data-testid="start-component">Start Component</div>,
}));

describe('CommonStaticStepFactory', () => {
  it('should render Start component when type is StaticStepTypes.Start', () => {
    render(<CommonStaticStepFactory type={StaticStepTypes.Start} />);

    expect(screen.getByTestId('start-component')).toBeInTheDocument();
    expect(screen.getByText('Start Component')).toBeInTheDocument();
  });

  it('should return null when type is StaticStepTypes.UserOnboard', () => {
    const {container} = render(<CommonStaticStepFactory type={StaticStepTypes.UserOnboard} />);

    expect(container.firstChild).toBeNull();
  });

  it('should return null for unknown static step type', () => {
    const {container} = render(<CommonStaticStepFactory type={'UNKNOWN_TYPE' as StaticStepTypes} />);

    expect(container.firstChild).toBeNull();
  });

  it('should render Start component with correct data-testid', () => {
    render(<CommonStaticStepFactory type={StaticStepTypes.Start} />);

    const startComponent = screen.getByTestId('start-component');
    expect(startComponent).toBeInTheDocument();
  });

  describe('StaticStepTypes values', () => {
    it('should have Start type defined', () => {
      expect(StaticStepTypes.Start).toBe('START');
    });

    it('should have UserOnboard type defined', () => {
      expect(StaticStepTypes.UserOnboard).toBe('USER_ONBOARD');
    });
  });
});
