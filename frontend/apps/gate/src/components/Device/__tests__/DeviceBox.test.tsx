// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DesignContext} from '@thunderid/design';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import DeviceBox from '../DeviceBox';

vi.mock('@thunderid/react', () => ({
  DeviceGrant: () => <div data-testid="device-grant">DeviceGrant</div>,
}));

describe('DeviceBox', () => {
  it('renders the Device Grant SDK component', () => {
    render(
      <DesignContext.Provider value={{isDesignEnabled: false, isLoading: false}}>
        <DeviceBox />
      </DesignContext.Provider>,
    );

    expect(screen.getByTestId('device-grant')).toBeInTheDocument();
  });
});
