// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Icon, type LucideProps} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import type {OxygenIconDefinition} from '../utils/loadOxygenIcons';

/**
 * Props of {@link OxygenIcon}: those of an icon component of `@wso2/oxygen-ui-icons-react`, and the
 * icon to draw.
 */
export interface OxygenIconProps extends LucideProps {
  /** The icon to draw, from the icon set {@link loadOxygenIcons} loads. */
  icon: OxygenIconDefinition;
}

/**
 * Draws an icon of `@wso2/oxygen-ui-icons-react` from the icon set {@link loadOxygenIcons} loads. It
 * renders the same markup as the icon's own component, through the Icon component the package's
 * icons are built on, and takes the same props.
 *
 * @param props - The icon and the props of an icon component.
 * @returns The icon.
 */
export default function OxygenIcon({icon, className = undefined, ...props}: OxygenIconProps): JSX.Element {
  // A component of the package adds its default class only when no class is passed, as here.
  const classes = [icon.className, className ?? icon.defaultClassName].filter((value) => value?.trim());

  return (
    <Icon
      iconNode={icon.iconNode}
      className={[...new Set(classes)].join(' ')}
      {...(icon.viewBox ? {viewBox: icon.viewBox} : {})}
      {...props}
    />
  );
}
