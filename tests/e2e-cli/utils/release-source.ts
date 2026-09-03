// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Release source
 *
 * The suite runs against one of two products: whatever is published on thunderid.dev, or a
 * distribution built in this repository. The second is what makes a product change provable: a
 * change to setup.sh, start.sh, deployment.yaml, the bootstrap resources or the sample bundle can
 * only break the CLI if the CLI is pointed at the build that contains it.
 *
 * Set E2E_PRODUCT_DIST to a directory of release zips (the shape of target/dist) and this serves
 * it with a generated manifest. The CLI reads it through THUNDERID_PRODUCT_VERSION, the same knob
 * an operator uses to point at their own mirror.
 */

import fs from "fs";
import http from "http";
import path from "path";

/** Fixed so the workers can reach the server global setup starts, without sharing state. */
export const RELEASE_SERVER_PORT = Number(process.env.E2E_RELEASE_SERVER_PORT ?? 8899);

/** The directory of release zips to serve, or null when running against the published product. */
export function localDistDir(): string | null {
  const dir = process.env.E2E_PRODUCT_DIST?.trim();
  return dir ? path.resolve(dir) : null;
}

/** The manifest URL the CLI should read, or null to leave it on the public one. */
export function localManifestURL(): string | null {
  return localDistDir() ? `http://127.0.0.1:${RELEASE_SERVER_PORT}/releases.json` : null;
}

interface Asset {
  name: string;
  downloadUrl: string;
}

/**
 * Builds a manifest from the zips present in dir.
 *
 * Naming is the contract: build.sh emits `thunderid-<version>-<platform>-<arch>.zip` and
 * `sample-app-<name>-<version>.zip`, which is exactly what release.PlatformAssetName and
 * release.SampleAssetName look for. The version is taken from the product zips, and the sample
 * zips are attached to the same release so `try` resolves against it.
 */
export function buildManifest(dir: string, baseUrl: string): { version: string; manifest: unknown } {
  const zips = fs.readdirSync(dir).filter(name => name.endsWith(".zip"));

  const productZips = zips.filter(name => /^thunderid-.+-(macos|linux|win)-(x64|arm64)\.zip$/.test(name));
  if (productZips.length === 0) {
    throw new Error(
      `No product zips in ${dir}. Expected something like thunderid-1.0.1-linux-x64.zip; found: ${
        zips.join(", ") || "(nothing)"
      }`
    );
  }

  const versions = new Set(
    productZips.map(name => /^thunderid-(.+)-(?:macos|linux|win)-(?:x64|arm64)\.zip$/.exec(name)![1])
  );
  if (versions.size > 1) {
    throw new Error(`${dir} holds more than one product version: ${[...versions].join(", ")}`);
  }
  const version = [...versions][0];

  // Sample zips carry their own version, so they are matched by prefix rather than by the
  // product's version.
  const assets: Asset[] = zips
    .filter(name => productZips.includes(name) || name.startsWith("sample-app-"))
    .map(name => ({ name, downloadUrl: `${baseUrl}/${name}` }));

  const release = { tagName: `v${version}`, isLatest: true, assets };
  return { version, manifest: { latestRelease: release, releases: [release] } };
}

/**
 * Serves dir over HTTP with a generated manifest at /releases.json.
 *
 * Returns a close function; global setup hands it back to Playwright as its teardown.
 */
export async function startDistServer(dir: string): Promise<() => Promise<void>> {
  const baseUrl = `http://127.0.0.1:${RELEASE_SERVER_PORT}`;
  const { version, manifest } = buildManifest(dir, baseUrl);
  const body = JSON.stringify(manifest);

  const server = http.createServer((req, res) => {
    const name = decodeURIComponent((req.url ?? "/").split("?")[0].replace(/^\//, ""));

    if (name === "releases.json") {
      res.writeHead(200, { "content-type": "application/json" });
      res.end(body);
      return;
    }

    // Serve only the zips in dir, by exact name: this stands in for a release CDN, not a file
    // browser, and a traversal here would read outside the distribution.
    const file = path.join(dir, path.basename(name));
    if (!name || path.basename(name) !== name || !fs.existsSync(file)) {
      res.writeHead(404).end("not found");
      return;
    }
    res.writeHead(200, { "content-type": "application/zip", "content-length": fs.statSync(file).size });
    fs.createReadStream(file).pipe(res);
  });

  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(RELEASE_SERVER_PORT, "127.0.0.1", resolve);
  });

  console.log(`📦 Serving ${path.relative(process.cwd(), dir)} as ThunderID v${version} on ${baseUrl}`);
  return () =>
    new Promise<void>(resolve => {
      server.closeAllConnections?.();
      server.close(() => resolve());
    });
}
