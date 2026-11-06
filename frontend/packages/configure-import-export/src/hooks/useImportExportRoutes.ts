// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * `home` is owned by the host (console implements it itself, not through a package), but this
 * package's pages navigate to it, so its shape is declared here too — narrowed to only the
 * members this package actually calls.
 *
 * The host supplies all of these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g.
 * this package rendered standalone in Storybook or a unit test), `useImportExportRoutes` falls
 * back to `defaultImportExportRoutePaths` below.
 *
 * @public
 */
export interface ImportExportRoutePaths {
  home: {
    list: () => string;
  };
  importExport: {
    list: () => string;
  };
  export: {
    page: () => string;
  };
  importConfiguration: {
    upload: () => string;
    validate: () => string;
    summary: () => string;
  };
}

/**
 * Default import/export paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultImportExportRoutePaths: ImportExportRoutePaths = {
  home: {
    list: () => '/home',
  },
  importExport: {
    list: () => '/import-export',
  },
  export: {
    page: () => '/export',
  },
  importConfiguration: {
    upload: () => '/import-configuration',
    validate: () => '/import-configuration/validate',
    summary: () => '/import-configuration/summary',
  },
};

/**
 * Resolves the import/export route paths, preferring the host application's
 * configuration (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode these destination paths; they should call this hook and build
 * the destination from the returned functions instead.
 *
 * @public
 */
export default function useImportExportRoutes(): ImportExportRoutePaths {
  const routes = useRoutes<Partial<ImportExportRoutePaths>>();
  return {
    home: routes.home ?? defaultImportExportRoutePaths.home,
    importExport: routes.importExport ?? defaultImportExportRoutePaths.importExport,
    export: routes.export ?? defaultImportExportRoutePaths.export,
    importConfiguration: routes.importConfiguration ?? defaultImportExportRoutePaths.importConfiguration,
  };
}
