/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import MDXComponents from '@theme-original/MDXComponents';
import {Box, Card, CardContent, Typography, ColorSchemeSVG, Table, TableBody, TableCell, TableContainer, TableHead, TableRow} from '@wso2/oxygen-ui';
import ApiReference from '@site/src/components/ApiReference';
import CodeBlock from '@site/src/components/CodeBlock';
import CodeGroup from '@site/src/components/CodeGroup';
import ColorSchemeImage from '@site/src/components/ColorSchemeImage';
import DocsGetStarted from '@site/src/components/DocsGetStarted';
import FloatingLogosBackground from '@site/src/components/FloatingLogosBackground';
import AndroidLogo from '@site/src/components/icons/AndroidLogo';
import AngularLogo from '@site/src/components/icons/AngularLogo';
import BrowserLogo from '@site/src/components/icons/BrowserLogo';
import ExpressLogo from '@site/src/components/icons/ExpressLogo';
import FlutterLogo from '@site/src/components/icons/FlutterLogo';
import GoLogo from '@site/src/components/icons/GoLogo';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import JavaScriptLogo from '@site/src/components/icons/JavaScriptLogo';
import NextLogo from '@site/src/components/icons/NextLogo';
import NodeLogo from '@site/src/components/icons/NodeLogo';
import NuxtLogo from '@site/src/components/icons/NuxtLogo';
import PythonLogo from '@site/src/components/icons/PythonLogo';
import ReactLogo from '@site/src/components/icons/ReactLogo';
import ReactRouterLogo from '@site/src/components/icons/ReactRouterLogo';
import TanStackLogo from '@site/src/components/icons/TanStackLogo';
import VueLogo from '@site/src/components/icons/VueLogo';
import {ConsoleUrl, WayFinderSampleUrl, WayFinderMailUrl} from '@site/src/components/LocalUrls';
import ProductName from '@site/src/components/ProductName';
import RepoLink from '@site/src/components/RepoLink';
import SDKCard from '@site/src/components/SDKCard';
import Stepper from '@site/src/components/Stepper';
import TutorialHero, {TutorialHeroItem} from '@site/src/components/TutorialHero';

export default {
  ...MDXComponents,
  Box,
  Card,
  CardContent,
  ColorSchemeSVG,
  ColorSchemeImage,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography,
  DocsGetStarted,
  Stepper,
  TutorialHero,
  TutorialHeroItem,
  SDKCard,
  ReactLogo,
  NextLogo,
  VueLogo,
  NuxtLogo,
  AngularLogo,
  BrowserLogo,
  NodeLogo,
  ExpressLogo,
  GoLogo,
  PythonLogo,
  FlutterLogo,
  IOSLogo,
  JavaScriptLogo,
  AndroidLogo,
  ReactRouterLogo,
  TanStackLogo,
  ApiReference,
  CodeBlock,
  CodeGroup,
  FloatingLogosBackground,
  ProductName,
  ConsoleUrl,
  WayFinderSampleUrl,
  WayFinderMailUrl,
  RepoLink,
  // TODO: Heading styling is a bit off when oxygen-ui Typography is used.
  // After sorting that out, we can switch to using Oxygen UI Typography for headings as well.
  // ex: h1: (props: TypographyProps<'h1'>) => <Typography variant="h1" {...props} />,
};
