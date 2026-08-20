// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig, type Plane} from '@thunderid/contexts';

export type {Plane};

/**
 * Returns the deployment plane this console instance serves, read from runtime config
 * (`window.__THUNDERID_RUNTIME_CONFIG__.plane`). Defaults to `'hybrid'` (show everything) when unset.
 */
export function usePlane(): Plane {
  const {config} = useConfig();

  return config.plane ?? 'hybrid';
}

/** Whether the given plane is the Control Plane authoring console. */
export function isControlPlane(plane: Plane): boolean {
  return plane === 'cp';
}

/**
 * A feature that is available on some planes but not others. Each feature lists the navigation entry
 * ids to hide and the top-level route path segments to block when it is not available. The nav-hiding
 * sets and the route-guard sets are all derived from these single sources.
 */
interface PlaneFeature {
  /** Navigation entry ids (parent and/or children) to hide when the feature is unavailable. */
  navIds?: string[];
  /** Top-level route path segments to block when the feature is unavailable (redirect to home). */
  routeSegments: string[];
}

/**
 * Data Plane runtime-only features, hidden on the Control Plane (authoring) console. The CP writes
 * declarative configuration only; these features operate on live runtime state (agent identities,
 * verifiable-credential issuance and presentation exchange, and the onboarding/tryout flow).
 */
// Agents and verifiable credentials are configuration like applications and connections: they are
// authored on a control plane and promoted to a data plane, so both planes show them.
const DP_ONLY_FEATURES: readonly PlaneFeature[] = [
  // Onboarding and tryout flow. No dedicated nav entry; reached via WelcomeRedirect and deep links.
  {routeSegments: ['welcome']},
];

/**
 * Control Plane authoring-only features, hidden on the Data Plane and hybrid consoles. Promotion and
 * the values it substitutes are Control Plane concerns: they describe what is applied to a Data Plane.
 */
const CP_ONLY_FEATURES: readonly PlaneFeature[] = [
  {navIds: ['promotions'], routeSegments: ['promotions']},
  {navIds: ['environment-variables'], routeSegments: ['environment-variables']},
];

const DP_ONLY_NAV_IDS = new Set<string>(DP_ONLY_FEATURES.flatMap((feature) => feature.navIds ?? []));
const CP_ONLY_NAV_IDS = new Set<string>(CP_ONLY_FEATURES.flatMap((feature) => feature.navIds ?? []));
const DP_ONLY_ROUTE_SEGMENTS_SET = new Set<string>(DP_ONLY_FEATURES.flatMap((feature) => feature.routeSegments));
const CP_ONLY_ROUTE_SEGMENTS_SET = new Set<string>(CP_ONLY_FEATURES.flatMap((feature) => feature.routeSegments));

/** Navigation entry ids hidden on the Control Plane console (Data Plane runtime-only features). */
export const CP_HIDDEN_NAV_IDS: ReadonlySet<string> = DP_ONLY_NAV_IDS;

/** Navigation entry ids hidden on the Data Plane and hybrid consoles (Control Plane-only features). */
export const CP_ONLY_HIDDEN_NAV_IDS: ReadonlySet<string> = CP_ONLY_NAV_IDS;

/** Top-level route path segments served only on the Data Plane (blocked on the Control Plane). */
export const DP_ONLY_ROUTE_SEGMENTS: ReadonlySet<string> = DP_ONLY_ROUTE_SEGMENTS_SET;

/** Top-level route path segments served only on the Control Plane (blocked on Data Plane/hybrid). */
export const CP_ONLY_ROUTE_SEGMENTS: ReadonlySet<string> = CP_ONLY_ROUTE_SEGMENTS_SET;

/** The navigation entry ids to hide for the given plane. */
export function hiddenNavIds(plane: Plane): ReadonlySet<string> {
  return isControlPlane(plane) ? DP_ONLY_NAV_IDS : CP_ONLY_NAV_IDS;
}

/** The first path segment of a location pathname, e.g. "/agents/create" -> "agents", "/" -> "". */
export function getTopLevelSegment(pathname: string): string {
  return pathname.split('/').find(Boolean) ?? '';
}

/**
 * Whether a route (by its top-level path segment) should be hidden on the given plane: Data Plane
 * runtime routes are blocked on the Control Plane, and Control Plane authoring routes are blocked on
 * the Data Plane and hybrid consoles.
 */
export function isRouteHiddenOnPlane(segment: string, plane: Plane): boolean {
  if (isControlPlane(plane)) {
    return DP_ONLY_ROUTE_SEGMENTS_SET.has(segment);
  }
  return CP_ONLY_ROUTE_SEGMENTS_SET.has(segment);
}
