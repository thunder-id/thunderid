// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

declare module '*.json' {
  const value: Record<string, unknown>;

  export default value;
}

declare module '*.svg' {
  import type {FunctionComponent, SVGProps} from 'react';

  export const ReactComponent: FunctionComponent<SVGProps<SVGSVGElement>>;
  const src: string;

  export default src;
}

declare module '*.png' {
  const content: string;

  export default content;
}

declare module '*.md';
