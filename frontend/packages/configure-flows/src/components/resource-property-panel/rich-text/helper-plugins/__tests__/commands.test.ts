// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect, vi} from 'vitest';

// Mock lexical's createCommand
const mockCreateCommand = vi.fn((name: string) => ({type: name}));
vi.mock('lexical', () => ({
  createCommand: mockCreateCommand,
}));

describe('commands', () => {
  it('should export TOGGLE_SAFE_LINK_COMMAND created with createCommand', async () => {
    // Import the module which will trigger createCommand
    const {default: TOGGLE_SAFE_LINK_COMMAND} = await import('../commands');

    // Verify createCommand was called with the correct command name
    expect(mockCreateCommand).toHaveBeenCalledWith('TOGGLE_SAFE_LINK_COMMAND');

    // Verify the exported command has the expected structure
    expect(TOGGLE_SAFE_LINK_COMMAND).toBeDefined();
    expect(TOGGLE_SAFE_LINK_COMMAND).toEqual({type: 'TOGGLE_SAFE_LINK_COMMAND'});
  });

  it('should be a LexicalCommand type for string payloads', async () => {
    const {default: TOGGLE_SAFE_LINK_COMMAND} = await import('../commands');

    // The command should be defined and usable
    expect(TOGGLE_SAFE_LINK_COMMAND).toBeDefined();
  });
});
