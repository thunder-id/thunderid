// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ComponentType} from 'react';
import {BackendForFrontendDiagram} from './diagrams';
import type {EcosystemDiagramName} from '@site/src/types/ecosystem';

/**
 * Typed as a total map over `EcosystemDiagramName`, so a name added to the type
 * without a component fails to compile. The build validates each `diagram:`
 * against the same names.
 */
const DIAGRAMS: Record<EcosystemDiagramName, ComponentType<{accent: string}>> = {
  'backend-for-frontend': BackendForFrontendDiagram,
};

export default DIAGRAMS;
