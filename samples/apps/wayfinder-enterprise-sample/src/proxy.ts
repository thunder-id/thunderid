// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { createRouteMatcher, thunderIDProxy } from '@thunderid/nextjs/server';

const isWorkspaceRoute = createRouteMatcher(['/workspace(.*)']);

// Keeps the session's tokens fresh and sends signed-out visitors from the workspace to the landing page.
export default thunderIDProxy(async (thunderid, request) => {
  if (isWorkspaceRoute(request)) {
    return thunderid.protectRoute({ redirect: '/' });
  }
  return undefined;
});

export const config = {
  matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};
