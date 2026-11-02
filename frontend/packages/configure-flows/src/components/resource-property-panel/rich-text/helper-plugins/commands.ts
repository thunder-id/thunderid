// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createCommand, type LexicalCommand} from 'lexical';

/**
 * Custom command for creating safe links.
 */
const TOGGLE_SAFE_LINK_COMMAND: LexicalCommand<string> = createCommand('TOGGLE_SAFE_LINK_COMMAND');

export default TOGGLE_SAFE_LINK_COMMAND;
