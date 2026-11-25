// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import * as IconPackage from '@wso2/oxygen-ui-icons-react';
import type {ComponentType, ReactNode} from 'react';
import {beforeAll, describe, expect, it} from 'vitest';
import loadOxygenIcons, {type OxygenIconSet} from '../../utils/loadOxygenIcons';
import OxygenIcon, {type OxygenIconProps} from '../OxygenIcon';

type IconComponent = ComponentType<Omit<OxygenIconProps, 'icon'>>;

// The icon components of the package, keyed by every name the package exports them under.
const iconComponents = Object.entries(IconPackage as Record<string, unknown>).filter(
  (entry): entry is [string, IconComponent] => {
    const value = entry[1];
    return (
      typeof value === 'object' &&
      value !== null &&
      '$$typeof' in value &&
      typeof (value as {displayName?: unknown}).displayName === 'string'
    );
  },
);

/**
 * Renders something for every icon component of the package, all at once, and returns the markup
 * rendered for each export name.
 */
function renderEach(draw: (name: string, Component: IconComponent) => ReactNode): Map<string, string> {
  const {container, unmount} = render(
    <>
      {iconComponents.map(([name, Component]) => (
        <div key={name}>{draw(name, Component)}</div>
      ))}
    </>,
  );
  const markup = new Map(iconComponents.map(([name], index) => [name, container.children[index].innerHTML]));
  unmount();
  return markup;
}

/** The export names whose icon OxygenIcon draws differently from the package's own component. */
function findMismatches(icons: OxygenIconSet, props: Omit<OxygenIconProps, 'icon'> = {}): string[] {
  const expected = renderEach((_name, Component) => <Component {...props} />);
  const actual = renderEach((name) => {
    const icon = icons.get(name);
    return icon ? <OxygenIcon icon={icon} {...props} /> : null;
  });
  return iconComponents.map(([name]) => name).filter((name) => actual.get(name) !== expected.get(name));
}

describe('OxygenIcon', () => {
  let icons: OxygenIconSet;

  // oxygenIcons.json is generated from the icon package (pnpm codegen:icons). These tests fail
  // when it no longer matches the installed package, and regenerating it fixes them.
  beforeAll(async () => {
    icons = await loadOxygenIcons();
  }, 30000);

  it('covers the icons of the package', () => {
    expect(iconComponents.length).toBeGreaterThan(1000);
  });

  it('lists each icon of the package once, under a name the package exports it under', () => {
    const listedComponents = icons.names.map((name) => (IconPackage as Record<string, unknown>)[name]);

    expect(new Set(listedComponents).size).toBe(icons.names.length);
    expect(new Set(listedComponents)).toEqual(new Set(iconComponents.map(([, component]) => component)));
  });

  it('draws every icon the same as its component of the package, under every name', () => {
    expect(findMismatches(icons)).toEqual([]);
  });

  it('applies the props of an icon component the same as the package does', () => {
    expect(
      findMismatches(icons, {
        size: 32,
        color: 'red',
        strokeWidth: 1,
        absoluteStrokeWidth: true,
        className: 'custom',
        'aria-label': 'Icon',
      }),
    ).toEqual([]);
  });
});
