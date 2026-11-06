// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useActiveVersion, useDocsPreferredVersion, useLatestVersion} from '@docusaurus/plugin-content-docs/client';

/**
 * Returns the `npx` command that installs ThunderID for the doc version the reader is in.
 *
 * The Next docs describe unreleased builds, so they install from the CLI's next channel
 * (`npx thunderid@next`); every released version installs the stable CLI (`npx thunderid`).
 * The version is resolved the same way `useDocsUrl` resolves it: the active version on a doc
 * route, then the reader's preferred version, then the latest release. This keeps a version-less
 * page such as the home page in step with the docs the reader is browsing.
 *
 * Doc pages write the same command as `npx {{npxPackage}}` in code blocks, which the build
 * resolves per version; this hook is the equivalent for React components.
 */
export function useNpxCommand(): string {
  const active = useActiveVersion(undefined);
  const {preferredVersion} = useDocsPreferredVersion(undefined);
  const latest = useLatestVersion(undefined);
  const version = active ?? preferredVersion ?? latest;
  return version.name === 'current' ? 'npx thunderid@next' : 'npx thunderid';
}
