// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import AndroidLogo from '../AndroidLogo';
import AppleIcon from '../AppleIcon';
import BunIcon from '../BunIcon';
import DenoIcon from '../DenoIcon';
import ExpressIcon from '../ExpressIcon';
import FlutterLogo from '../FlutterLogo';
import GithubIcon from '../GithubIcon';
import GoogleIcon from '../GoogleIcon';
import HeidiIcon from '../HeidiIcon';
import JavaScriptIcon from '../JavaScriptIcon';
import JsonLogo from '../JsonLogo';
import JwtLogo from '../JwtLogo';
import LangChainLogo from '../LangChainLogo';
import LissiIcon from '../LissiIcon';
import NextjsIcon from '../NextjsIcon';
import NodeIcon from '../NodeIcon';
import NpmIcon from '../NpmIcon';
import NuxtIcon from '../NuxtIcon';
import OAuth2Logo from '../OAuth2Logo';
import PnpmIcon from '../PnpmIcon';
import PythonLogo from '../PythonLogo';
import ReactIcon from '../ReactIcon';
import StackblitzIcon from '../StackblitzIcon';
import TypeScriptLogo from '../TypeScriptLogo';
import VueIcon from '../VueIcon';
import YarnIcon from '../YarnIcon';

function renderSvg(element: React.ReactElement): SVGSVGElement {
  const {container} = render(element);
  const svg = container.querySelector('svg');
  if (!svg) {
    throw new Error('Expected component to render an <svg> element');
  }
  return svg;
}

function widthOf(svg: SVGSVGElement): number {
  return Number(svg.getAttribute('width'));
}

function heightOf(svg: SVGSVGElement): number {
  return Number(svg.getAttribute('height'));
}

interface SquareIconCase {
  Component: (props: {size?: number}) => React.ReactElement;
  defaultSize: number;
  name: string;
}

const SQUARE_ICONS: SquareIconCase[] = [
  {Component: AppleIcon, defaultSize: 20, name: 'AppleIcon'},
  {Component: ExpressIcon, defaultSize: 20, name: 'ExpressIcon'},
  {Component: GithubIcon, defaultSize: 20, name: 'GithubIcon'},
  {Component: GoogleIcon, defaultSize: 20, name: 'GoogleIcon'},
  {Component: JavaScriptIcon, defaultSize: 20, name: 'JavaScriptIcon'},
  {Component: LangChainLogo, defaultSize: 64, name: 'LangChainLogo'},
  {Component: NextjsIcon, defaultSize: 20, name: 'NextjsIcon'},
  {Component: NuxtIcon, defaultSize: 20, name: 'NuxtIcon'},
  {Component: OAuth2Logo, defaultSize: 36, name: 'OAuth2Logo'},
  {Component: ReactIcon, defaultSize: 20, name: 'ReactIcon'},
  {Component: NpmIcon, defaultSize: 20, name: 'NpmIcon'},
  {Component: YarnIcon, defaultSize: 20, name: 'YarnIcon'},
  {Component: PnpmIcon, defaultSize: 20, name: 'PnpmIcon'},
  {Component: BunIcon, defaultSize: 20, name: 'BunIcon'},
  {Component: DenoIcon, defaultSize: 20, name: 'DenoIcon'},
];

describe.each(SQUARE_ICONS)('$name', ({Component, defaultSize}) => {
  it('renders an svg with equal width and height at the default size', () => {
    const svg = renderSvg(<Component />);

    expect(widthOf(svg)).toBe(defaultSize);
    expect(heightOf(svg)).toBe(defaultSize);
  });

  it('scales width and height together when a custom size is passed', () => {
    const svg = renderSvg(<Component size={48} />);

    expect(widthOf(svg)).toBe(48);
    expect(heightOf(svg)).toBe(48);
  });
});

interface AspectIconCase {
  Component: (props: {size?: number}) => React.ReactElement;
  defaultSize: number;
  heightFromSize: (size: number) => number;
  name: string;
  widthFromSize: (size: number) => number;
}

const ASPECT_ICONS: AspectIconCase[] = [
  {
    Component: AndroidLogo,
    defaultSize: 64,
    heightFromSize: (size) => size * 0.586,
    name: 'AndroidLogo',
    widthFromSize: (size) => size,
  },
  {
    Component: FlutterLogo,
    defaultSize: 64,
    heightFromSize: (size) => size * 1.238,
    name: 'FlutterLogo',
    widthFromSize: (size) => size,
  },
  {
    Component: NodeIcon,
    defaultSize: 20,
    heightFromSize: (size) => (size * 50) / 44,
    name: 'NodeIcon',
    widthFromSize: (size) => size,
  },
  {
    Component: PythonLogo,
    defaultSize: 64,
    heightFromSize: (size) => size * 1.018,
    name: 'PythonLogo',
    widthFromSize: (size) => size,
  },
  {
    Component: TypeScriptLogo,
    defaultSize: 64,
    heightFromSize: (size) => size,
    name: 'TypeScriptLogo',
    widthFromSize: (size) => size,
  },
  {
    Component: StackblitzIcon,
    defaultSize: 18,
    heightFromSize: (size) => size,
    name: 'StackblitzIcon',
    widthFromSize: (size) => (size * 89) / 18,
  },
  {
    Component: JsonLogo,
    defaultSize: 18,
    heightFromSize: (size) => size,
    name: 'JsonLogo',
    widthFromSize: (size) => (size * 108) / 32,
  },
  {
    Component: JwtLogo,
    defaultSize: 20,
    heightFromSize: (size) => size,
    name: 'JwtLogo',
    widthFromSize: (size) => (size * 90) / 32,
  },
  {
    Component: VueIcon,
    defaultSize: 20,
    heightFromSize: (size) => (size * 170.02) / 196.32,
    name: 'VueIcon',
    widthFromSize: (size) => size,
  },
];

describe.each(ASPECT_ICONS)('$name', ({Component, defaultSize, heightFromSize, widthFromSize}) => {
  it('preserves its aspect ratio at the default size', () => {
    const svg = renderSvg(<Component />);

    expect(widthOf(svg)).toBeCloseTo(widthFromSize(defaultSize), 5);
    expect(heightOf(svg)).toBeCloseTo(heightFromSize(defaultSize), 5);
  });

  it('preserves its aspect ratio when a custom size is passed', () => {
    const svg = renderSvg(<Component size={defaultSize * 2} />);

    expect(widthOf(svg)).toBeCloseTo(widthFromSize(defaultSize * 2), 5);
    expect(heightOf(svg)).toBeCloseTo(heightFromSize(defaultSize * 2), 5);
  });
});

describe('HeidiIcon', () => {
  it('renders an svg sized by height at the default height', () => {
    const svg = renderSvg(<HeidiIcon />);

    expect(heightOf(svg)).toBe(24);
  });

  it('honors a custom height', () => {
    const svg = renderSvg(<HeidiIcon height={48} />);

    expect(heightOf(svg)).toBe(48);
  });
});

describe('LissiIcon', () => {
  it('renders an svg sized by height at the default height', () => {
    const svg = renderSvg(<LissiIcon />);

    expect(heightOf(svg)).toBe(24);
  });

  it('honors a custom height', () => {
    const svg = renderSvg(<LissiIcon height={48} />);

    expect(heightOf(svg)).toBe(48);
  });
});

describe('JavaScriptIcon', () => {
  it('exposes an accessible name via role and aria-label', () => {
    const svg = renderSvg(<JavaScriptIcon />);

    expect(svg.getAttribute('role')).toBe('img');
    expect(svg.getAttribute('aria-label')).toBe('JavaScript logo');
  });
});
