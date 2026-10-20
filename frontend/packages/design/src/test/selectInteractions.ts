// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, fireEvent, screen} from '@testing-library/react';

/** Opens the select under test by pressing the mouse on its combobox. */
export const openList = (): void => {
  fireEvent.mouseDown(screen.getByRole('combobox'));
};

/** Lets pending timers and the promises behind them run, for assertions about something that must not happen. */
export const settle = async (): Promise<void> => {
  await act(async () => {
    await new Promise((resolve) => {
      setTimeout(resolve, 100);
    });
  });
};
