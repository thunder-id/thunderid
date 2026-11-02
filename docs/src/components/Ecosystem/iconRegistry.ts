// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AndroidLogo, FlutterLogo} from '@thunderid/components';
import {ComponentType} from 'react';
import {EcosystemIconName} from './iconNames';
import AngularLogo from '../icons/AngularLogo';
import AuthJsLogo from '../icons/AuthJsLogo';
import BetterAuthLogo from '../icons/BetterAuthLogo';
import BrowserLogo from '../icons/BrowserLogo';
import ClaudeLogo from '../icons/ClaudeLogo';
import CodexLogo from '../icons/CodexLogo';
import ExpressLogo from '../icons/ExpressLogo';
import GoLogo from '../icons/GoLogo';
import IOSLogo from '../icons/IOSLogo';
import JavaScriptLogo from '../icons/JavaScriptLogo';
import NextLogo from '../icons/NextLogo';
import NodeLogo from '../icons/NodeLogo';
import NuxtAuthLogo from '../icons/NuxtAuthLogo';
import NuxtLogo from '../icons/NuxtLogo';
import PassportLogo from '../icons/PassportLogo';
import PythonLogo from '../icons/PythonLogo';
import ReactLogo from '../icons/ReactLogo';
import ReactRouterLogo from '../icons/ReactRouterLogo';
import SkillsLogo from '../icons/SkillsLogo';
import SpringSecurityLogo from '../icons/SpringSecurityLogo';
import TanStackLogo from '../icons/TanStackLogo';
import VueLogo from '../icons/VueLogo';

export type EcosystemIcon = ComponentType<{size?: number}>;

/**
 * Resolves the `icon:` string in an `sdk.yaml` to a component.
 *
 * The registry is typed as a total map over `EcosystemIconName`, so adding a
 * name to `iconNames.ts` without wiring a component here is a compile error
 * rather than a blank card.
 */
const ECOSYSTEM_ICONS: Record<EcosystemIconName, EcosystemIcon> = {
  AndroidLogo,
  AngularLogo,
  AuthJsLogo,
  BetterAuthLogo,
  BrowserLogo,
  ClaudeLogo,
  CodexLogo,
  ExpressLogo,
  FlutterLogo,
  GoLogo,
  IOSLogo,
  JavaScriptLogo,
  NextLogo,
  NodeLogo,
  NuxtAuthLogo,
  NuxtLogo,
  PassportLogo,
  PythonLogo,
  ReactLogo,
  ReactRouterLogo,
  SkillsLogo,
  SpringSecurityLogo,
  TanStackLogo,
  VueLogo,
};

/**
 * The build validates every `icon:` against `ECOSYSTEM_ICON_NAMES`, so a miss
 * here means the registry and the name list have drifted. Returning undefined
 * lets the caller render a neutral placeholder instead of crashing the page.
 */
export default function getEcosystemIcon(name: string): EcosystemIcon | undefined {
  return ECOSYSTEM_ICONS[name as EcosystemIconName];
}
