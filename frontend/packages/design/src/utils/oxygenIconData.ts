// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// The icon data, behind a module of its own that loadOxygenIcons imports dynamically. Importing the
// JSON file dynamically instead makes rolldown export its top-level members by name, and it drops
// their declarations, which breaks the build of the apps.
import oxygenIcons from './oxygenIcons.json';

export default oxygenIcons;
