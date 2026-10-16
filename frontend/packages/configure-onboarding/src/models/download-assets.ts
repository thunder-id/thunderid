// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Type definitions for the welcome page release assets data.
 */
export interface ReleaseAssetInput {
  downloadUrl: string;
  name: string;
  sizeLabel: string;
}

/**
 * Type definition for the response of the Wayfinder configuration import API.
 */
export interface ReleaseEntry {
  assets: ReleaseAssetInput[];
  tagName: string;
}

/**
 * Type definition for the welcome page releases data, which includes the latest release and a list of all releases.
 */
export interface ReleasesData {
  latestRelease: ReleaseEntry | null;
  releases: ReleaseEntry[];
}
