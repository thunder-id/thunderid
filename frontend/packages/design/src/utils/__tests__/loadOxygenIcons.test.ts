// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import loadOxygenIcons, {getLoadedOxygenIcons} from '../loadOxygenIcons';

describe('loadOxygenIcons', () => {
  it('loads the icon set once and caches it', async () => {
    const first = await loadOxygenIcons();
    const second = await loadOxygenIcons();

    expect(second).toBe(first);
    expect(getLoadedOxygenIcons()).toBe(first);
  }, 30000);

  it('lists each icon once, sorted', async () => {
    const {names} = await loadOxygenIcons();

    expect(names).toContain('User');
    expect(names).not.toContain('UserIcon');
    expect(names).not.toContain('LucideUser');
    expect([...names].sort((a, b) => a.localeCompare(b))).toEqual(names);
  });

  it('resolves the other names an icon is exported under to the name it is listed under', async () => {
    const icons = await loadOxygenIcons();

    expect(icons.resolveName('User')).toBe('User');
    expect(icons.resolveName('UserIcon')).toBe('User');
    expect(icons.resolveName('LucideUser')).toBe('User');
    // A renamed icon, by its former name.
    expect(icons.resolveName('Fingerprint')).toBe('FingerprintPattern');
    expect(icons.resolveName('FingerprintIcon')).toBe('FingerprintPattern');
  });

  it('resolves names the package does not export as icons to nothing', async () => {
    const icons = await loadOxygenIcons();

    expect(icons.resolveName('NotAnIcon')).toBeUndefined();
    expect(icons.resolveName('NotAnIconIcon')).toBeUndefined();
    expect(icons.resolveName('Icon')).toBeUndefined();
    expect(icons.resolveName('toString')).toBeUndefined();
    expect(icons.get('constructor')).toBeUndefined();
  });

  it('gives the same definition for every name of an icon', async () => {
    const icons = await loadOxygenIcons();
    const user = icons.get('User');

    expect(user).toMatchObject({name: 'User', className: 'lucide-user'});
    expect(icons.get('UserIcon')).toBe(user);
  });

  it('gives the custom icons of the package their own class and viewBox', async () => {
    const icons = await loadOxygenIcons();

    expect(icons.get('Google')).toMatchObject({name: 'Google', defaultClassName: 'lucide-logo'});
  });
});
