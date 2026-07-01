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

import Link from '@docusaurus/Link';
import {Box} from '@wso2/oxygen-ui';
import {
  BarChart3,
  Briefcase,
  CircleUser,
  ClipboardCheck,
  Code2,
  Globe,
  LayoutGrid,
  LockKeyhole,
  LogIn,
  Palette,
  Share2,
  ShieldCheck,
  ShieldPlus,
  UserPlus,
  UserX,
} from '@wso2/oxygen-ui-icons-react';
import React from 'react';
import './B2CIdentityJourney.css';

import {
  UseCaseBuildingBlockDetail,
  UseCaseBuildingBlockPanel,
  UseCaseBuildingBlocksExplorer,
} from './UseCaseBuildingBlocksExplorer';
import { UseCaseCapabilityMap, UseCaseMapGroup, UseCaseMapNode } from './UseCaseCapabilityMap';

export { UseCaseBuildingBlockPanel };

// ─── Shared icon containers (already sx, kept as-is) ───────────────────────

const iconContainerSx = {
  width: '3.4rem',
  minWidth: '3.4rem',
  height: '3.4rem',
  borderRadius: '50%',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 38%, var(--ifm-color-emphasis-300))',
  background: `radial-gradient(68px 68px at 28% 18%, color-mix(in srgb, var(--ifm-color-primary) 24%, transparent), transparent),
    linear-gradient(160deg, color-mix(in srgb, var(--ifm-color-primary) 72%, #091629), color-mix(in srgb, var(--ifm-color-primary) 44%, #030712))`,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 24%, transparent), 0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 20%, transparent)',
  '& svg': {
    width: '1.55rem',
    height: '1.55rem',
    stroke: '#fff',
    fill: 'none',
    strokeWidth: '1.8',
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  },
} as const;

const supportingIconContainerSx = {
  ...iconContainerSx,
  width: '2.5rem',
  minWidth: '2.5rem',
  height: '2.5rem',
  flex: 'none',
  aspectRatio: '1',
} as const;

// ─── uc-approach-cards ──────────────────────────────────────────────────────

const approachCardsSx = {
  display: 'flex',
  flexDirection: 'column',
  gap: '1rem',
  margin: '1.25rem 0',
} as const;

// ─── uc-b2c-roadmap ─────────────────────────────────────────────────────────

const roadmapSx = {
  '--uc-node-w': '9rem',
  '--uc-node-h': '5rem',
  '--uc-icon-size': '4rem',
  display: 'flex',
  flexWrap: 'wrap',
  justifyContent: 'center',
  alignItems: 'flex-start',
  gap: '1.25rem 1.4rem',
  margin: '1.5rem 0 2rem',
  padding: '0.5rem 0.2rem',
  '@media (max-width: 640px)': {
    '--uc-node-w': '7.5rem',
    '--uc-node-h': '4.4rem',
    '--uc-icon-size': '3.4rem',
    gap: '1rem 0.8rem',
  },
} as const;

const roadmapNodeSx = {
  width: 'var(--uc-node-w)',
  minHeight: 'var(--uc-node-h)',
  border: 0,
  background: 'transparent',
  cursor: 'pointer',
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'flex-start',
  gap: '0.55rem',
  textAlign: 'center',
  textDecoration: 'none',
  color: 'var(--ifm-font-color-base)',
  fontWeight: 700,
  fontSize: '0.82rem',
  lineHeight: 1.2,
  transition: 'transform 160ms ease',
  '&:hover': {
    transform: 'translateY(-2px)',
    textDecoration: 'none',
  },
  '&:focus-visible': {
    outline: '2px solid color-mix(in srgb, var(--ifm-color-primary) 58%, white)',
    outlineOffset: '4px',
    borderRadius: '6px',
  },
  '&:hover .uc-b2c-roadmap__icon': {
    borderColor: 'color-mix(in srgb, #ffffff 56%, var(--ifm-color-primary))',
    boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 32%, transparent), 0 12px 24px color-mix(in srgb, var(--ifm-color-primary) 34%, transparent)',
  },
} as const;

const roadmapIconSx = {
  width: 'var(--uc-icon-size)',
  minWidth: 'var(--uc-icon-size)',
  height: 'var(--uc-icon-size)',
  borderRadius: '999px',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 38%, var(--ifm-color-emphasis-300))',
  background: `radial-gradient(80px 80px at 28% 18%, color-mix(in srgb, var(--ifm-color-primary) 24%, transparent), transparent),
    linear-gradient(160deg, color-mix(in srgb, var(--ifm-color-primary) 72%, #091629), color-mix(in srgb, var(--ifm-color-primary) 44%, #030712))`,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  boxShadow: 'inset 0 0 0 1px color-mix(in srgb, #fff 24%, transparent), 0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 24%, transparent)',
  '& svg': {
    width: '1.75rem',
    height: '1.75rem',
    stroke: '#fff',
    fill: 'none',
    strokeWidth: '1.8',
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  },
} as const;

const roadmapLabelSx = {
  display: 'block',
  maxWidth: '9.4rem',
} as const;

// ─── uc-solution-map ────────────────────────────────────────────────────────

const solutionMapSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 20%, var(--ifm-color-emphasis-200))',
  borderRadius: '12px',
  background: `radial-gradient(700px 280px at 20% 0%, color-mix(in srgb, var(--ifm-color-primary) 9%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 28%, transparent)`,
  display: 'grid',
  gap: '0.9rem',
  gridTemplateColumns: 'minmax(0, 1fr)',
  margin: '1.25rem 0 1.75rem',
  overflow: 'hidden',
  padding: '1.1rem',
  position: 'relative',
  '@media (max-width: 640px)': {
    padding: '0.85rem',
  },
} as const;

const solutionMapRailSx = {
  background: 'color-mix(in srgb, var(--ifm-color-primary) 36%, var(--ifm-color-emphasis-300))',
  height: '2px',
  left: '2.5rem',
  position: 'absolute',
  right: '2.5rem',
  top: '2.05rem',
  '@media (max-width: 900px)': {
    display: 'none',
  },
  '@media (max-width: 640px)': {
    bottom: 'auto',
    display: 'block',
    height: 'auto',
    left: '1.9rem',
    right: 'auto',
    top: '1.8rem',
    width: '2px',
  },
} as const;

const solutionMapStagesSx = {
  display: 'grid',
  gap: '0.7rem',
  gridTemplateColumns: 'repeat(4, minmax(0, 1fr))',
  position: 'relative',
  '@media (max-width: 900px)': {
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  },
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

const solutionMapStageSx = {
  appearance: 'none',
  background: 'transparent',
  border: 0,
  color: 'inherit',
  cursor: 'pointer',
  font: 'inherit',
  minWidth: 0,
  padding: 0,
  position: 'relative',
  textAlign: 'left',
  '&:focus-visible': {
    outline: '2px solid color-mix(in srgb, var(--ifm-color-primary) 60%, white)',
    outlineOffset: '4px',
  },
  '&:hover .uc-sm-content, &[aria-selected="true"] .uc-sm-content': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 42%, var(--ifm-color-emphasis-300))',
    boxShadow: '0 10px 24px color-mix(in srgb, var(--ifm-color-primary) 12%, transparent)',
    transform: 'translateY(-1px)',
  },
  '@media (max-width: 640px)': {
    display: 'grid',
    gap: '0.55rem',
    gridTemplateColumns: '2.1rem minmax(0, 1fr)',
  },
} as const;

const solutionMapIndexSx = {
  alignItems: 'center',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 13%, var(--ifm-background-color))',
  border: '2px solid color-mix(in srgb, var(--ifm-color-primary) 48%, var(--ifm-color-emphasis-300))',
  borderRadius: '999px',
  color: 'color-mix(in srgb, var(--ifm-color-primary) 88%, var(--ifm-font-color-base))',
  display: 'flex',
  fontSize: '0.72rem',
  fontWeight: 850,
  height: '2.1rem',
  justifyContent: 'center',
  margin: '0 auto 0.6rem',
  position: 'relative',
  width: '2.1rem',
  zIndex: 1,
  '@media (max-width: 640px)': {
    margin: 0,
  },
} as const;

const solutionMapIndexActiveSx = {
  ...solutionMapIndexSx,
  background: 'color-mix(in srgb, var(--ifm-color-primary) 88%, #111827)',
  color: 'white',
} as const;

const solutionMapContentSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 16%, var(--ifm-color-emphasis-200))',
  borderRadius: '8px',
  background: 'color-mix(in srgb, var(--ifm-background-color) 78%, transparent)',
  minHeight: '100%',
  padding: '0.85rem',
  transition: 'border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease',
  '@media (max-width: 640px)': {
    padding: '0.72rem',
  },
} as const;

const solutionMapContentTitleSx = {
  display: 'block',
  fontSize: '0.9rem',
  fontWeight: 800,
  lineHeight: 1.25,
  margin: '0 0 0.35rem',
} as const;

const solutionMapContentDescSx = {
  color: 'var(--ifm-color-emphasis-800)',
  display: 'block',
  fontSize: '0.76rem',
  lineHeight: 1.4,
  margin: '0 0 0.55rem',
} as const;

const solutionMapSelectionSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 18%, var(--ifm-color-emphasis-300))',
  borderRadius: '999px',
  color: 'color-mix(in srgb, var(--ifm-color-primary) 88%, var(--ifm-font-color-base))',
  display: 'inline-flex',
  fontSize: '0.7rem',
  fontWeight: 800,
  lineHeight: 1.15,
  padding: '0.24rem 0.42rem',
} as const;

const solutionMapDetailSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 18%, var(--ifm-color-emphasis-200))',
  borderRadius: '10px',
  background: 'color-mix(in srgb, var(--ifm-background-color) 78%, transparent)',
  display: 'grid',
  gap: '1rem',
  gridTemplateColumns: 'minmax(0, 0.98fr) minmax(16rem, 0.8fr)',
  padding: '1rem',
  '@media (max-width: 900px)': {
    gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  },
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

const solutionMapOptionsSx = {
  display: 'grid',
  gap: '0.55rem',
} as const;

const solutionMapOptionSx = {
  appearance: 'none',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 16%, var(--ifm-color-emphasis-300))',
  borderRadius: '8px',
  background: 'color-mix(in srgb, var(--ifm-color-emphasis-100) 22%, transparent)',
  color: 'inherit',
  cursor: 'pointer',
  display: 'grid',
  gap: '0.22rem',
  font: 'inherit',
  padding: '0.7rem 0.75rem',
  textAlign: 'left',
  transition: 'background 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease',
  '& strong': {
    color: 'var(--ifm-font-color-base)',
    fontSize: '0.86rem',
    lineHeight: 1.2,
  },
  '& span': {
    color: 'var(--ifm-color-emphasis-800)',
    fontSize: '0.75rem',
    lineHeight: 1.35,
  },
  '&:hover': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 46%, var(--ifm-color-emphasis-300))',
    transform: 'translateY(-1px)',
  },
} as const;

const solutionMapOptionActiveSx = {
  ...solutionMapOptionSx,
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 46%, var(--ifm-color-emphasis-300))',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 9%, transparent)',
  boxShadow: '0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 12%, transparent)',
  transform: 'translateY(-1px)',
} as const;

const graphicBackgrounds: Record<string, string> = {
  integration: `linear-gradient(160deg, color-mix(in srgb, var(--ifm-color-primary) 9%, transparent), transparent 55%),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 18%, transparent)`,
  'identity-data': `linear-gradient(160deg, color-mix(in srgb, #0f766e 12%, transparent), transparent 55%),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 18%, transparent)`,
  'tokens-apis': `linear-gradient(160deg, color-mix(in srgb, #7c3aed 10%, transparent), transparent 55%),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 18%, transparent)`,
  operations: `linear-gradient(160deg, color-mix(in srgb, #b45309 10%, transparent), transparent 55%),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 18%, transparent)`,
};

const solutionMapGraphicBaseSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 18%, var(--ifm-color-emphasis-200))',
  borderRadius: '8px',
  display: 'grid',
  gap: '0.9rem',
  padding: '0.85rem',
  '& ul': {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '0.35rem',
    listStyle: 'none',
    margin: 0,
    padding: 0,
  },
  '& li': {
    border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 15%, var(--ifm-color-emphasis-300))',
    borderRadius: '999px',
    color: 'var(--ifm-color-emphasis-900)',
    fontSize: '0.7rem',
    fontWeight: 750,
    lineHeight: 1.15,
    padding: '0.24rem 0.42rem',
  },
} as const;

const solutionMapGraphicFlowSx = {
  alignItems: 'center',
  display: 'grid',
  gap: '0.45rem',
  gridTemplateColumns: 'minmax(0, 1fr) auto minmax(0, 1.15fr) auto minmax(0, 1fr)',
  minHeight: '6.6rem',
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
    minHeight: 0,
  },
} as const;

const solutionMapGraphicNodeSx = {
  alignItems: 'center',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 20%, var(--ifm-color-emphasis-300))',
  borderRadius: '8px',
  background: 'color-mix(in srgb, var(--ifm-background-color) 80%, transparent)',
  color: 'var(--ifm-font-color-base)',
  display: 'flex',
  fontSize: '0.78rem',
  fontWeight: 800,
  justifyContent: 'center',
  lineHeight: 1.25,
  minHeight: '4.8rem',
  padding: '0.55rem',
  textAlign: 'center',
  '@media (max-width: 640px)': {
    minHeight: '3.7rem',
  },
} as const;

const solutionMapGraphicNodePrimarySx = {
  ...solutionMapGraphicNodeSx,
  background: `radial-gradient(110px 80px at 50% 0%, color-mix(in srgb, var(--ifm-color-primary) 18%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-primary) 9%, var(--ifm-background-color))`,
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 42%, var(--ifm-color-emphasis-300))',
} as const;

const solutionMapGraphicArrowSx = {
  color: 'color-mix(in srgb, var(--ifm-color-primary) 70%, var(--ifm-font-color-base))',
  fontSize: '1.15rem',
  fontWeight: 850,
  '@media (max-width: 640px)': {
    textAlign: 'center',
    transform: 'rotate(90deg)',
  },
} as const;

const solutionMapCrossCuttingSx = {
  alignItems: 'center',
  border: '1px solid color-mix(in srgb, #0f766e 24%, var(--ifm-color-emphasis-200))',
  borderRadius: '8px',
  background: `radial-gradient(260px 160px at 100% 0%, color-mix(in srgb, #0f766e 10%, transparent), transparent),
    color-mix(in srgb, var(--ifm-background-color) 80%, transparent)`,
  display: 'grid',
  gap: '0.75rem',
  gridTemplateColumns: 'minmax(10rem, 0.28fr) minmax(0, 1fr)',
  padding: '0.85rem',
  '& h3': {
    fontSize: '0.9rem',
    lineHeight: 1.25,
    margin: '0 0 0.25rem',
  },
  '& p': {
    color: 'var(--ifm-color-emphasis-800)',
    fontSize: '0.76rem',
    lineHeight: 1.4,
    margin: 0,
  },
  '& ul': {
    display: 'flex',
    flexWrap: 'wrap',
    gap: '0.35rem',
    listStyle: 'none',
    margin: 0,
    padding: 0,
  },
  '& li': {
    border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 15%, var(--ifm-color-emphasis-300))',
    borderRadius: '999px',
    color: 'var(--ifm-color-emphasis-900)',
    fontSize: '0.7rem',
    fontWeight: 750,
    lineHeight: 1.15,
    padding: '0.24rem 0.42rem',
  },
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

// ─── uc-identity-sources-diagram ────────────────────────────────────────────

const identitySourcesDiagramSx = {
  alignItems: 'center',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 24%, var(--ifm-color-emphasis-200))',
  borderRadius: '12px',
  background: `radial-gradient(760px 260px at 50% 0%, color-mix(in srgb, var(--ifm-color-primary) 9%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 22%, transparent)`,
  boxShadow: '0 16px 34px rgba(15, 23, 42, 0.08)',
  display: 'flex',
  flexDirection: 'column',
  gap: 0,
  margin: '1.25rem 0 2rem',
  padding: '1.4rem 1.1rem 1.1rem',
} as const;

const identitySourcesHeroSx = {
  alignItems: 'center',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 30%, var(--ifm-color-emphasis-200))',
  borderRadius: '10px',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 8%, var(--ifm-background-color))',
  display: 'flex',
  flexDirection: 'column',
  gap: '0.4rem',
  maxWidth: '18rem',
  padding: '0.8rem 1.2rem',
  textAlign: 'center',
  width: '100%',
  '& strong': {
    fontSize: '0.95rem',
    fontWeight: 850,
    lineHeight: 1.15,
  },
} as const;

const identitySourcesHeroIconSx = {
  '& svg': {
    display: 'block',
    fill: 'none',
    height: '2.4rem',
    margin: '0 auto',
    stroke: 'color-mix(in srgb, var(--ifm-color-primary) 80%, var(--ifm-font-color-base))',
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    strokeWidth: 1.5,
    width: '2.4rem',
  },
} as const;

const identitySourcesForkSx = {
  color: 'color-mix(in srgb, var(--ifm-color-primary) 36%, var(--ifm-color-emphasis-300))',
  height: '2rem',
  maxWidth: '28rem',
  width: '60%',
  '& svg': {
    display: 'block',
    fill: 'none',
    height: '100%',
    stroke: 'currentColor',
    strokeLinecap: 'round' as const,
    strokeWidth: 1.5,
    width: '100%',
  },
  '@media (max-width: 640px)': {
    display: 'none',
  },
} as const;

const identitySourcesQuestionsSx = {
  alignItems: 'start',
  display: 'grid',
  gap: '0.8rem',
  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  width: '100%',
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

const identitySourcesQuestionSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 18%, var(--ifm-color-emphasis-200))',
  borderRadius: '10px',
  background: 'color-mix(in srgb, var(--ifm-color-emphasis-100) 20%, transparent)',
  display: 'flex',
  flexDirection: 'column',
  gap: '0.8rem',
  padding: '0.8rem',
} as const;

const identitySourcesEyebrowSx = {
  color: 'color-mix(in srgb, var(--ifm-color-primary) 86%, var(--ifm-font-color-base))',
  display: 'block',
  fontSize: '0.7rem',
  fontWeight: 850,
  letterSpacing: '0.08em',
  lineHeight: 1.1,
  marginBottom: '0.25rem',
  textTransform: 'uppercase',
} as const;

const identitySourcesQuestionHeadSx = {
  '& h3': {
    fontSize: '0.98rem',
    margin: '0 0 0.2rem',
  },
  '& h3 a': {
    color: 'inherit',
    textDecoration: 'none',
  },
  '& h3 a:hover': {
    textDecoration: 'underline',
  },
  '& p': {
    color: 'var(--ifm-color-emphasis-800)',
    fontSize: '0.78rem',
    lineHeight: 1.35,
    margin: 0,
  },
} as const;

const identitySourcesItemGroupSx = {
  display: 'grid',
  gap: '0.45rem',
  '& h4': {
    color: 'var(--ifm-color-emphasis-700)',
    fontSize: '0.72rem',
    fontWeight: 800,
    letterSpacing: '0.04em',
    margin: 0,
    textTransform: 'uppercase',
  },
} as const;

const identitySourcesItemsSx = {
  display: 'flex',
  flexDirection: 'column',
  gap: '0.45rem',
  listStyle: 'none',
  margin: 0,
  padding: 0,
  '& li': {
    alignItems: 'flex-start',
    border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 14%, var(--ifm-color-emphasis-200))',
    borderRadius: '8px',
    background: 'color-mix(in srgb, var(--ifm-background-color) 72%, transparent)',
    display: 'flex',
    gap: '0.55rem',
    padding: '0.5rem 0.6rem',
  },
  '& li strong': {
    display: 'block',
    fontSize: '0.82rem',
    fontWeight: 850,
    lineHeight: 1.2,
  },
  '& li span > span': {
    color: 'var(--ifm-color-emphasis-700)',
    display: 'block',
    fontSize: '0.74rem',
    lineHeight: 1.3,
  },
} as const;

const identitySourcesItemIconSx = {
  flexShrink: 0,
  marginTop: '0.1rem',
  '& svg': {
    display: 'block',
    fill: 'none',
    height: '1.1rem',
    stroke: 'color-mix(in srgb, var(--ifm-color-primary) 70%, var(--ifm-color-emphasis-500))',
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    strokeWidth: 1.5,
    width: '1.1rem',
  },
} as const;

// ─── uc-solution-chooser ────────────────────────────────────────────────────

const solutionChooserSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 24%, var(--ifm-color-emphasis-200))',
  borderRadius: '12px',
  background: `radial-gradient(760px 260px at 50% 0%, color-mix(in srgb, var(--ifm-color-primary) 9%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-emphasis-100) 22%, transparent)`,
  boxShadow: '0 16px 34px rgba(15, 23, 42, 0.08)',
  margin: '1.25rem 0 2rem',
  padding: '1.1rem',
} as const;

const solutionChooserQuestionsSx = {
  display: 'grid',
  gap: '0.8rem',
  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

const solutionChooserQuestionSx = {
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 18%, var(--ifm-color-emphasis-200))',
  borderRadius: '10px',
  background: 'color-mix(in srgb, var(--ifm-color-emphasis-100) 20%, transparent)',
  padding: '0.8rem',
  '& h3': {
    fontSize: '0.98rem',
    margin: '0 0 0.25rem',
  },
  '& p': {
    color: 'var(--ifm-color-emphasis-800)',
    fontSize: '0.78rem',
    lineHeight: 1.35,
    margin: '0 0 0.72rem',
  },
} as const;

const solutionChooserEyebrowSx = {
  color: 'color-mix(in srgb, var(--ifm-color-primary) 86%, var(--ifm-font-color-base))',
  display: 'block',
  fontSize: '0.7rem',
  fontWeight: 850,
  letterSpacing: '0.08em',
  lineHeight: 1.1,
  marginBottom: '0.25rem',
  textTransform: 'uppercase',
} as const;

const solutionChooserOptionsSx = {
  display: 'grid',
  gap: '0.45rem',
  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  '@media (max-width: 640px)': {
    gridTemplateColumns: '1fr',
  },
} as const;

const solutionChooserOptionSx = {
  appearance: 'none',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 22%, var(--ifm-color-emphasis-300))',
  borderRadius: '8px',
  background: 'color-mix(in srgb, var(--ifm-background-color) 72%, transparent)',
  color: 'var(--ifm-font-color-base)',
  cursor: 'pointer',
  font: 'inherit',
  fontSize: '0.82rem',
  fontWeight: 800,
  lineHeight: 1.2,
  minHeight: '2.4rem',
  padding: '0.42rem 0.55rem',
  textAlign: 'center',
  transition: 'background 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease',
  '&:hover': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 48%, var(--ifm-color-emphasis-300))',
    transform: 'translateY(-1px)',
  },
  '&:disabled': {
    cursor: 'not-allowed',
    opacity: 0.48,
    transform: 'none',
  },
} as const;

const solutionChooserOptionActiveSx = {
  ...solutionChooserOptionSx,
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 48%, var(--ifm-color-emphasis-300))',
  background: `radial-gradient(100px 64px at 50% 0%, color-mix(in srgb, var(--ifm-color-primary) 18%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-primary) 10%, transparent)`,
  boxShadow: '0 8px 18px color-mix(in srgb, var(--ifm-color-primary) 14%, transparent)',
  transform: 'translateY(-1px)',
} as const;

const solutionChooserPatternsSx = {
  display: 'flex',
  gap: '0.7rem',
  justifyContent: 'center',
  margin: '1.05rem 0',
  '@media (max-width: 640px)': {
    alignItems: 'stretch',
    flexDirection: 'column',
  },
} as const;

const solutionChooserRecommendedSx = {
  borderRadius: '999px',
  background: 'color-mix(in srgb, var(--ifm-color-primary) 16%, transparent)',
  color: 'color-mix(in srgb, var(--ifm-color-primary) 88%, var(--ifm-font-color-base))',
  fontSize: '0.62rem',
  fontWeight: 850,
  letterSpacing: '0.05em',
  lineHeight: 1,
  padding: '0.2rem 0.42rem',
  textTransform: 'uppercase',
} as const;


const solutionChooserRecLinkSx = {
  alignItems: 'center',
  color: 'var(--ifm-color-primary)',
  display: 'inline-flex',
  fontSize: '0.82rem',
  fontWeight: 700,
  gap: '0.3rem',
  marginTop: '0.85rem',
  textDecoration: 'none',
  transition: 'gap 140ms ease',
  '&:hover': {
    gap: '0.5rem',
    textDecoration: 'none',
  },
} as const;

// ─── uc-glass-card ──────────────────────────────────────────────────────────

const glassCardSx = {
  background: 'color-mix(in srgb, var(--ifm-color-primary) 6%, var(--ifm-background-surface-color, var(--ifm-card-background)))',
  border: '1px solid color-mix(in srgb, var(--ifm-color-primary) 22%, var(--ifm-color-emphasis-200))',
  borderRadius: '8px',
  backdropFilter: 'blur(10px)',
  WebkitBackdropFilter: 'blur(10px)',
  '[data-theme="dark"] &': {
    background: 'color-mix(in srgb, var(--ifm-color-primary) 12%, rgba(255, 255, 255, 0.045))',
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 28%, rgba(255, 255, 255, 0.1))',
  },
} as const;

// ─── uc-arch-decisions ──────────────────────────────────────────────────────

const archDecisionsSx = {
  containerName: 'architecture-decisions',
  containerType: 'inline-size',
  margin: '1.5rem 0 2rem',
  width: '100%',
} as const;

const archDecisionsGridSx = {
  display: 'grid',
  gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 13.5rem), 1fr))',
  gap: '0.75rem',
} as const;

const archDecisionsPrioritizedSx = {
  ...archDecisionsSx,
  display: 'grid',
  gap: '1.25rem',
  maxWidth: '48rem',
} as const;

const archDecisionsStepSx = {
  color: 'var(--ifm-color-primary)',
  fontSize: '0.72rem',
  fontWeight: 800,
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
} as const;

const archDecisionsPrimarySx = {
  display: 'grid',
  gap: '0.55rem',
  '& .uc-arch-decision-card--primary': {
    alignItems: 'center',
    display: 'grid',
    gap: '0.9rem',
    gridTemplateColumns: 'auto minmax(0, 1fr)',
  },
} as const;

const archDecisionsSupportingSx = {
  display: 'grid',
  gap: '0.55rem',
} as const;

const archDecisionsSupportingGridSx = {
  alignItems: 'stretch',
  display: 'grid',
  gap: '0.75rem',
  gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
} as const;

// ─── uc-arch-decision-card ──────────────────────────────────────────────────

const archDecisionCardBaseSx = {
  ...glassCardSx,
  color: 'var(--ifm-font-color-base)',
  display: 'flex',
  flexDirection: 'column',
  gap: '0.5rem',
  minWidth: 0,
  padding: '1rem',
  textDecoration: 'none',
  transition: 'border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease',
  '&:hover': {
    borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 58%, var(--ifm-color-emphasis-300))',
    boxShadow: '0 8px 20px color-mix(in srgb, var(--ifm-color-primary) 14%, transparent)',
    transform: 'translateY(-2px)',
    textDecoration: 'none',
    color: 'var(--ifm-font-color-base)',
  },
} as const;

const archDecisionCardPrimarySx = {
  ...archDecisionCardBaseSx,
  background: `radial-gradient(300px 160px at 0% 0%, color-mix(in srgb, var(--ifm-color-primary) 18%, transparent), transparent),
    color-mix(in srgb, var(--ifm-color-primary) 8%, var(--ifm-background-surface-color, var(--ifm-card-background)))`,
  borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 48%, var(--ifm-color-emphasis-300))',
} as const;

const archDecisionCardSupportingSx = {
  ...archDecisionCardBaseSx,
  alignSelf: 'stretch',
  padding: '0.8rem',
} as const;

const archDecisionCardBodySx = {
  display: 'grid',
  gap: '0.25rem',
} as const;

const archDecisionCardBodyPrimarySx = {
  ...archDecisionCardBodySx,
  display: 'flex',
  flex: 1,
  flexDirection: 'column',
} as const;

const archDecisionCardBodySupportingSx = {
  ...archDecisionCardBodySx,
  display: 'flex',
  flex: 1,
  flexDirection: 'column',
} as const;

const archDecisionCardTitleSx = {
  fontSize: '0.95rem',
  fontWeight: 700,
  lineHeight: 1.2,
} as const;

const archDecisionCardTitleSupportingSx = {
  ...archDecisionCardTitleSx,
  fontSize: '0.86rem',
} as const;

const archDecisionCardQuestionSx = {
  color: 'var(--ifm-color-emphasis-700)',
  fontSize: '0.8rem',
  lineHeight: 1.45,
  margin: 0,
  flex: 1,
} as const;

const archDecisionCardQuestionSupportingSx = {
  ...archDecisionCardQuestionSx,
  fontSize: '0.74rem',
  lineHeight: 1.35,
  flex: 'none',
} as const;

const archDecisionCardCtaSx = {
  fontSize: '0.8rem',
  fontWeight: 600,
  color: 'var(--ifm-color-primary)',
  marginTop: '0.25rem',
} as const;

const archDecisionCardCtaPrimarySx = {
  ...archDecisionCardCtaSx,
  marginTop: 'auto',
  paddingTop: '0.25rem',
} as const;

const archDecisionCardCtaSupportingSx = {
  ...archDecisionCardCtaSx,
  fontSize: '0.76rem',
  marginTop: 'auto',
  paddingTop: 0,
} as const;

// ─── uc-next-steps ──────────────────────────────────────────────────────────

const nextStepsSx = {
  margin: '2rem 0',
  display: 'flex',
  flexDirection: 'column',
  gap: '1.5rem',
} as const;

const nextStepsTrySx = {
  ...glassCardSx,
  display: 'block',
  borderRadius: '12px !important',
  borderWidth: '1.5px !important',
  padding: '1.75rem 2rem',
  textDecoration: 'none !important',
  color: 'inherit !important',
  transition: 'border-color 0.15s ease, box-shadow 0.15s ease',
  '&:hover': {
    borderColor: 'var(--ifm-color-primary) !important',
    boxShadow: '0 4px 20px color-mix(in srgb, var(--ifm-color-primary) 18%, transparent)',
  },
} as const;

const nextStepsTryEyebrowSx = {
  fontSize: '0.7rem',
  fontWeight: 700,
  textTransform: 'uppercase',
  letterSpacing: '0.1em',
  color: 'var(--ifm-color-primary)',
  marginBottom: '0.4rem',
} as const;

const nextStepsTryTitleSx = {
  fontSize: '1.1rem',
  fontWeight: 700,
  color: 'var(--ifm-font-color-base)',
  marginBottom: '0.5rem',
} as const;

const nextStepsTryDescSx = {
  fontSize: '0.9rem',
  color: 'var(--ifm-color-emphasis-700)',
  margin: '0 0 1.1rem',
} as const;

const nextStepsTryBtnSx = {
  display: 'inline-block',
  background: 'var(--ifm-color-primary)',
  color: '#fff !important',
  padding: '0.5rem 1.15rem',
  borderRadius: '6px',
  fontWeight: 600,
  fontSize: '0.875rem',
  textDecoration: 'none !important',
} as const;

// ────────────────────────────────────────────────────────────────────────────

interface RoadmapNode {
  href: string;
  label: string;
  icon: React.ReactNode;
}

const roadmapNodes: RoadmapNode[] = [
  {
    href: '#b2c-identity-journey',
    label: 'Sign In',
    icon: <LogIn />,
  },
  {
    href: '#enable-self-sign-up',
    label: 'Self Sign-Up',
    icon: <UserPlus />,
  },
  {
    href: '#add-self-service-profile-management',
    label: 'Manage Profile',
    icon: <CircleUser />,
  },
  {
    href: '#add-account-recovery',
    label: 'Recover Access',
    icon: <LockKeyhole />,
  },
  {
    href: '#onboard-internal-users',
    label: 'Internal Users',
    icon: <Briefcase />,
  },
  {
    href: '#handle-account-closure',
    label: 'Close Accounts',
    icon: <UserX />,
  },
  {
    href: '#defend-against-abuse-and-risk',
    label: 'Defend Against Abuse',
    icon: <ShieldCheck />,
  },
  {
    href: '#gain-identity-insights',
    label: 'Identity Insights',
    icon: <BarChart3 />,
  },
];

const roadmapIcons = roadmapNodes.map((node) => node.icon);

const journeyDetails: UseCaseBuildingBlockDetail[] = [
  {
    id: 'sign-in',
    label: 'Sign in',
    title: 'Add Sign-In to Your Application',
    icon: roadmapIcons[0],
    why:
      'As your most visible identity surface, sign-in needs to feel effortless. Consumers expect to choose the method they already prefer, such as password, social sign-in, passkey, or passwordless sign-in.',
    example: (
      <>
        A user installs your mobile application, taps <strong>Sign in with Google</strong>, and reaches their dashboard within seconds. Later, when they try to change their email address, the application asks them to confirm a one-time code as a step-up check. A power user enables a passkey on their phone and from then on signs in with a single tap of their passkey - no password required.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Authentication methods',
        items: [
          'Password sign-in',
          'Email or SMS one-time code',
          'Magic link',
          'Passkey',
          'Social sign-in',
          'Enterprise identity provider sign-in',
        ],
      },
      {
        title: 'Security controls',
        items: [
          'Multi-factor authentication',
          'Step-up authentication',
          'Persistent sign-in / remember me',
        ],
      },
    ],
  },
  {
    id: 'self-sign-up',
    label: 'Self sign-up',
    title: 'Enable Self Sign-Up',
    icon: roadmapIcons[1],
    why:
      'New users decide whether your product is worth their time in the first minute. Use self sign-up when users should be able to create accounts without administrator involvement.',
    example: (
      <>
        A new user lands on your home page, taps <strong>Sign up with Google</strong>, and arrives signed in with a basic profile already filled in. Their consent decisions are recorded during sign-up and can be revisited later from settings.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Registration methods',
        items: [
          'Email and password sign-up',
          'Passwordless sign-up',
          'Social sign-up',
          'Passkey-first registration',
          'Just-in-time account creation',
        ],
      },
      {
        title: 'Profile and trust',
        items: [
          'Progressive profile collection',
          'Email or phone verification',
          'Terms and marketing consent capture',
        ],
      },
    ],
  },
  {
    id: 'manage-profile',
    label: 'Manage profile',
    title: 'Manage Customer Profile',
    icon: roadmapIcons[2],
    why:
      'Once a consumer signs in, they expect a self-service area where they can view and change their own identity without contacting support. Profile management reduces support load and improves user trust.',
    example: (
      <>
        A user opens the account page and switches from password to passkey. They also enable two-factor authentication - the application asks for their phone number and verifies it via a one-time code before activating it. They remove an old linked Google account they no longer use and sign out a session on a device they sold last month. When they change their email address, the new address is verified via a magic link before the change takes effect.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Account details',
        items: [
          'View and edit profile attributes',
          'Update verified email or phone with re-verification',
          'Manage linked social and enterprise identities',
        ],
      },
      {
        title: 'Security and privacy',
        items: [
          'Change password',
          'Add or remove a passkey',
          'Manage second factors',
          'View active sessions and revoke a specific session',
          'View and withdraw stored consents',
          'Account deletion or data export',
        ],
      },
    ],
  },
  {
    id: 'recover-access',
    label: 'Recover access',
    title: 'Recover Customer Access',
    icon: roadmapIcons[3],
    why:
      'When users lose access to their account, it tests whether they stay or leave. Recovery paths should be quick for legitimate users and resistant to account takeover.',
    example: (
      <>
        A user who forgot their password requests a magic link delivered to their email, clicks it, sets a new password, and is signed back in. Another user who lost their phone uses an email one-time code for recovery because their SMS channel is no longer verified. A third, whose account was locked after too many failed attempts, is automatically unlocked after the lockout window expires.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Recovery methods',
        items: [
          'Forgotten-password reset',
          'Email magic link',
          'Email one-time code',
          'SMS one-time code',
        ],
      },
      {
        title: 'Recovery controls',
        items: [
          'Recovery channel verification',
          'Account unlock after lockout',
        ],
      },
    ],
  },
  {
    id: 'internal-users',
    label: 'Internal users',
    title: 'Manage Internal Users',
    icon: roadmapIcons[4],
    why:
      'Behind every consumer product is a team keeping it running. Support agents, administrators, and operations staff need a separate onboarding path with identity and role decided before they arrive.',
    example: (
      <>
        A new support agent receives an invitation email and follows the link. They set a password, accept the support terms, and land in the admin console with the support role pre-assigned. Separately, an operations admin creates ten new staff accounts directly for a regional support team. The initial passwords are auto-generated and distributed via a secure channel; the staff members rotate them on first sign-in.
      </>
    ),
    capabilityGroups: [
      {
        title: 'User creation',
        items: [
          'Invite a user by email',
          'Create a user account directly with initial credentials',
          'Bulk invite or bulk create',
          'Configurable invitation expiry and resend',
          'Revoke a pending invitation',
        ],
      },
      {
        title: 'Access setup',
        items: [
          'Onboarding after invitation acceptance',
          'Credential setup on first sign-in',
          'Initial role or permission assignment',
        ],
      },
    ],
  },
  {
    id: 'account-closure',
    label: 'Close accounts',
    title: 'Handle Account Closure',
    icon: roadmapIcons[5],
    why:
      'Accounts have an end as well as a beginning. Users decide to leave and expect a clear way to close their account. You need to remove accounts that violate your terms. Inactive accounts pile up over time and need to age out.',
    example: (
      <>
        A user closes their account from settings, and the account and its data are removed in line with your retention policy. A separate account is suspended by an admin after a fraud flag, with the reason captured against the record. An inactive account that has not been touched for two years receives a warning email, then is expired when the user does not return.
      </>
    ),
    capabilityGroups: [
      {
        title: 'User-initiated',
        items: [
          'Self-service account closure',
          'Audit record of the closure event',
        ],
      },
      {
        title: 'Admin-initiated',
        items: [
          'Account suspension with reason capture',
          'Inactive-account detection and expiry',
          'Prior notification before expiry',
        ],
      },
    ],
  },
  {
    id: 'defend-against-abuse',
    label: 'Defend against abuse',
    title: 'Defend Against Abuse and Risk',
    icon: roadmapIcons[6],
    why:
      'Identity flows attract abuse from day one. Bots try to mass-create accounts, attackers run credential stuffing, and even legitimate users can land in risky situations. The level of friction should adapt to the risk in the moment.',
    example: (
      <>
        A sign-up wave from a single IP range hits a bot challenge and is throttled before any accounts are created. A user signing in from a new country is asked for a one-time code as a step-up, then completes sign-in normally. A repeated wrong-password pattern triggers a temporary lockout, while genuine sign-ins continue to succeed.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Detection',
        items: [
          'Bot detection on sign-up and sign-in',
          'CAPTCHA or invisible challenge integration',
          'Rate limiting per user, IP address, and device',
          'Credential stuffing and brute-force detection',
          'Risk signals: new device, new location, impossible travel',
        ],
      },
      {
        title: 'Response',
        items: [
          'Adaptive step-up authentication based on risk',
          'Account lockout with automatic unlock',
        ],
      },
    ],
  },
  {
    id: 'identity-insights',
    label: 'Identity insights',
    title: 'Gain Identity Insights',
    icon: roadmapIcons[7],
    why:
      'Identity is one of the highest-signal touch points your application has with each user. Without visibility, you optimize sign-up in the dark, miss security incidents until they escalate, and scramble when compliance asks.',
    example: (
      <>
        A product manager notices the sign-up completion rate dropped after a new terms screen went live. A security lead receives an alert about a spike in failed sign-ins from one IP range. A support agent reviews a user&apos;s recent sign-in attempts to diagnose an access issue. A compliance officer exports the audit log for an annual review.
      </>
    ),
    capabilityGroups: [
      {
        title: 'Analytics',
        items: [
          'Sign-up and sign-in funnel analytics with drop-off points',
          'Adoption metrics by authentication method',
          'Active user trends and registration over time',
        ],
      },
      {
        title: 'Audit and security',
        items: [
          'Audit log of identity events',
          'Per-user activity history for support investigations',
          'Security signals and real-time alerts',
          'Stream identity events to external analytics and SIEM tools',
          'Export audit data for compliance reporting',
        ],
      },
    ],
  },
];

const crossCuttingIcons = {
  federation: <Share2 />,
  authorization: <LayoutGrid />,
  consent: <ClipboardCheck />,
  branding: <Palette />,
  localization: <Globe />,
  privacy: <ShieldPlus />,
};

const crossCuttingDetails: UseCaseBuildingBlockDetail[] = [
  {
    id: 'federated-identity',
    label: 'Federation',
    title: 'Federated Identity',
    icon: crossCuttingIcons.federation,
    why:
      'External identity providers, both social and enterprise, let users bring an identity they already have to your app. Done well, federation creates one user record per real person regardless of how many sign-in methods they use.',
    capabilityGroups: [
      {
        title: 'Identity providers',
        items: [
          'Social identity provider sign-in',
          'Enterprise OIDC identity provider sign-in',
          'Connected identity sign-out behavior',
        ],
      },
      {
        title: 'Account mapping',
        items: [
          'Just-in-time account creation',
          'Account linking',
          'Federated profile mapping',
        ],
      },
    ],
  },
  {
    id: 'authorization',
    label: 'Authorization',
    title: 'Authorization',
    icon: crossCuttingIcons.authorization,
    why:
      'When your app calls APIs on behalf of the user, it needs the right level of access. Scopes describe what the app may do and audiences describe which API the token is valid for.',
    capabilityGroups: [
      {
        title: 'Token controls',
        items: [
          'OAuth2 scopes',
          'Audience-restricted tokens',
          'Claims for application decisions',
        ],
      },
      {
        title: 'Access decisions',
        items: [
          'Role-aware access where needed',
          'API authorization',
          'Least-privilege access requests',
        ],
      },
    ],
  },
  {
    id: 'consent',
    label: 'Consent',
    title: 'Consent',
    icon: crossCuttingIcons.consent,
    why:
      'Where authorization describes what the app requests, consent is where the user agrees to it. Consent decisions should be recorded so users can review or revoke them later.',
    capabilityGroups: [
      {
        title: 'Consent capture',
        items: [
          'Profile-sharing consent',
          'Permission consent',
          'Terms of service acceptance',
          'Privacy policy acceptance',
          'Marketing preference capture',
        ],
      },
      {
        title: 'Consent lifecycle',
        items: [
          'Consent review and revocation',
          'Consent records for audit',
        ],
      },
    ],
  },
  {
    id: 'branding',
    label: 'Branding',
    title: 'Branding',
    icon: crossCuttingIcons.branding,
    why:
      'Your sign-in, sign-up, and recovery surfaces should match your brand whether they live on hosted pages or inside your own app screens.',
    capabilityGroups: [
      {
        title: 'Visual identity',
        items: [
          'Hosted page branding',
          'App-native screen consistency',
          'Logo and color customization',
          'Branded copy',
        ],
      },
      {
        title: 'Experience consistency',
        items: [
          'Localized sign-in experience',
          'Recovery flow branding',
        ],
      },
    ],
  },
  {
    id: 'localization',
    label: 'Localization',
    title: 'Localization',
    icon: crossCuttingIcons.localization,
    why:
      "Identity surfaces should speak the user's language. Sign-in, sign-up, recovery, and profile screens render in the user's locale, with right-to-left layouts where the language needs them.",
    capabilityGroups: [
      {
        title: 'Language and layout',
        items: [
          'Locale-aware identity screens',
          'Right-to-left layout support',
          'Localized notification emails and SMS',
        ],
      },
      {
        title: 'Accessibility',
        items: [
          'Keyboard navigation',
          'Screen reader support',
          'Sufficient contrast and accessibility standards',
        ],
      },
    ],
  },
  {
    id: 'privacy',
    label: 'Privacy',
    title: 'Privacy',
    icon: crossCuttingIcons.privacy,
    why:
      'Consent capture and policy alignment should be built into registration and profile interactions, not bolted on later.',
    capabilityGroups: [
      {
        title: 'Data visibility',
        items: [
          'Stored data visibility',
          'Shared data visibility',
          'Consent history',
        ],
      },
      {
        title: 'User controls',
        items: [
          'Account deletion workflows',
          'Data export workflows',
          'Privacy preference management',
        ],
      },
    ],
  },
];

const b2cRootNode: UseCaseMapNode = {
  id: 'add-login',
  href: '#add-login-to-your-application',
  label: 'Add Sign-In',
  icon: roadmapIcons[0],
};

const b2cUseCaseGroups: UseCaseMapGroup[] = [
  {
    id: 'identity-access',
    label: 'Identity & Access',
    nodes: [
      {
        id: 'self-sign-up',
        href: '#enable-self-sign-up',
        label: 'Enable Self Sign-Up',
        icon: roadmapIcons[1],
      },
      {
        id: 'recover-access',
        href: '#add-account-recovery',
        label: 'Configure Account Recovery',
        icon: roadmapIcons[3],
      },
      {
        id: 'federated-identity',
        href: '#federated-identity',
        label: 'Add Federated Sign-In',
        icon: crossCuttingIcons.federation,
      },
      {
        id: 'authorization',
        href: '#authorization',
        label: 'Authorize Access',
        icon: crossCuttingIcons.authorization,
      },
    ],
  },
  {
    id: 'administration',
    label: 'Administration',
    nodes: [
      {
        id: 'manage-profile',
        href: '#add-self-service-profile-management',
        label: 'Enable Profile Management',
        icon: roadmapIcons[2],
      },
      {
        id: 'internal-users',
        href: '#onboard-internal-users',
        label: 'Manage Internal Team Access',
        icon: roadmapIcons[4],
      },
      {
        id: 'account-closure',
        href: '#handle-account-closure',
        label: 'Handle Account Closure',
        icon: roadmapIcons[5],
      },
    ],
  },
  {
    id: 'configuration',
    label: 'Configuration',
    nodes: [
      {
        id: 'consent',
        href: '#consent',
        label: 'Manage Consent',
        icon: crossCuttingIcons.consent,
      },
      {
        id: 'branding',
        href: '#branding',
        label: 'Customize Branding',
        icon: crossCuttingIcons.branding,
      },
      {
        id: 'localization',
        href: '#localization',
        label: 'Localize Identity Surfaces',
        icon: crossCuttingIcons.localization,
      },
    ],
  },
  {
    id: 'operations',
    label: 'Operations',
    nodes: [
      {
        id: 'defend-against-abuse',
        href: '#defend-against-abuse-and-risk',
        label: 'Defend Against Abuse',
        icon: roadmapIcons[6],
      },
      {
        id: 'identity-insights',
        href: '#gain-identity-insights',
        label: 'Gain Identity Insights',
        icon: roadmapIcons[7],
      },
      {
        id: 'privacy',
        href: '#privacy',
        label: 'Protect Customer Data',
        icon: crossCuttingIcons.privacy,
      },
    ],
  },
];

export function B2CIdentityUseCaseMap() {
  return (
    <UseCaseCapabilityMap
      ariaLabel="B2C identity use case capability map"
      root={b2cRootNode}
      groups={b2cUseCaseGroups}
    />
  );
}

export function B2CIdentityJourneyExplorer() {
  return (
    <>
      <h3>Primary B2C Journeys</h3>
      <p>Each block below represents a distinct identity use case. Select one to see what it covers and which capabilities are involved.</p>
      <UseCaseBuildingBlocksExplorer
        ariaLabel="Primary B2C identity journeys"
        detailPanelId="b2c-journey-detail"
        groups={[
          {
            id: 'primary-journeys',
            nodes: journeyDetails,
          },
        ]}
      />
      <h3>Cross-Cutting Capabilities</h3>
      <p>These capabilities are not tied to a single journey. They apply across the identity system and are relevant to most B2C applications.</p>
      <UseCaseBuildingBlocksExplorer
        ariaLabel="Cross-cutting B2C identity capabilities"
        detailPanelId="b2c-cross-cutting-detail"
        groups={[
          {
            id: 'cross-cutting-capabilities',
            nodes: crossCuttingDetails,
            variant: 'secondary',
          },
        ]}
      />
    </>
  );
}

const solutionPatternNodes: RoadmapNode[] = [
  {
    href: '#redirect-based',
    label: 'Redirect-Based',
    icon: <LogIn />,
  },
  {
    href: '#app-native',
    label: 'App-Native',
    icon: <LayoutGrid />,
  },
  {
    href: '#direct-api',
    label: 'Direct API',
    icon: <Code2 />,
  },
];

const solutionPatternDetails: UseCaseBuildingBlockDetail[] = [
  {
    id: 'redirect-based',
    label: 'Redirect-based',
    title: 'Redirect-Based',
    icon: solutionPatternNodes[0].icon,
    why:
      'ThunderID hosts the identity screens. Your app redirects users there and gets them back signed in.',
  },
  {
    id: 'app-native',
    label: 'App-native',
    title: 'App-Native',
    icon: solutionPatternNodes[1].icon,
    why:
      'Your app renders every screen, but ThunderID owns the journey — step ordering, branching, and policy stay on the server.',
  },
  {
    id: 'direct-api',
    label: 'Direct API',
    title: 'Direct API',
    icon: solutionPatternNodes[2].icon,
    why:
      "Your app calls ThunderID's primitive APIs directly — low-level, single-purpose operations with no hosted pages and no journey to configure. You decide what to call, when to call it, and what to do with the result.",
  },
];

export function B2CIntegrationApproachesCards() {
  return (
    <Box sx={approachCardsSx}>
      {solutionPatternDetails.map((pattern) => (
        <UseCaseBuildingBlockPanel
          key={pattern.id}
          icon={pattern.icon}
          title={pattern.title}
          why={pattern.why}
          capabilityGroups={pattern.capabilityGroups}
        />
      ))}
    </Box>
  );
}

const tokensAndApisDetails: UseCaseBuildingBlockDetail[] = [
  {
    id: 'session-token-strategy',
    label: 'Session and Token Strategy',
    title: 'Session and Token Strategy',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l3 3" />
      </svg>
    ),
    why: 'Once a user signs in, the application holds a token or session that represents them. The shape of that credential decides how long the user stays signed in, how quickly access can be revoked when needed, and how much load lands on the identity product. Most B2C apps make this decision once and live with it for years, so it pays to pick deliberately.',
    capabilityGroups: [
      {
        title: 'Patterns',
        items: [
          'Stateless tokens: short-lived access tokens backed by longer refresh tokens, with no server-side session record. Scales easily; revocation waits for the token to expire.',
          'Server-backed sessions: every refresh is backed by a server-side record the identity product can revoke instantly, enabling true sign-out-everywhere.',
          'Sliding-expiry sessions: sessions extend on each use so returning users rarely have to sign in again, in return for longer-lived credentials.',
        ],
      },
      {
        title: 'Capabilities',
        items: [
          'Stateless JWT access tokens with refresh tokens',
          'Server-side session record backing each refresh',
          'Instant token and session revocation',
          'Refresh-token rotation on use',
          'Sliding-expiry sessions for stay-signed-in',
          'Configurable access, refresh, and session lifetimes',
          'Single logout across application surfaces',
        ],
      },
    ],
  },
  {
    id: 'protect-apis',
    label: 'Protect APIs',
    title: 'Protect APIs the App Calls',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M12 2 3 7v6c0 5.5 3.8 10.7 9 12 5.2-1.3 9-6.5 9-12V7L12 2z" />
        <path d="m9 12 2 2 4-4" />
      </svg>
    ),
    why: "Sign-in is one half of identity; the other is protecting the APIs your app calls after sign-in. The identity product issues tokens during sign-in, and the same tokens carry the user's permissions to your APIs. The API does not need to know who the user is; it only needs to validate the token and check the permissions inside.",
    capabilityGroups: [
      {
        title: 'Capabilities',
        items: [
          'Issue an OAuth2 access token to the app on sign-in',
          'Validate the token at the API edge via JWT signature verification or introspection',
          'Scope-based authorization checks',
          'Bind a token to a specific API via audience or resource indicator',
          'Group related APIs into a resource server so they share permission rules',
        ],
      },
    ],
  },
];

/** @deprecated use B2CIntegrationApproachesCards */
export const B2CIntegrationApproachesRoadmap = B2CIntegrationApproachesCards;

export function B2CTokensAndApisCards() {
  return (
    <Box sx={approachCardsSx}>
      {tokensAndApisDetails.map((item) => (
        <UseCaseBuildingBlockPanel
          key={item.id}
          icon={item.icon}
          title={item.title}
          why={item.why}
          capabilityGroups={item.capabilityGroups}
        />
      ))}
    </Box>
  );
}

const operationsDetails: UseCaseBuildingBlockDetail[] = [
  {
    id: 'identity-as-code',
    label: 'Identity-as-Code',
    title: 'Identity-as-Code',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <path d="M14 2v6h6" />
        <path d="M9 15 7 13l2-2" />
        <path d="m13 11 2 2-2 2" />
      </svg>
    ),
    why: 'Identity configuration spans many resources such as user types, applications, roles, flows, federated providers, and branding, and it grows over time. Managing it by hand breaks down as soon as you have separate dev, staging, and production environments. Identity-as-Code treats configuration as versioned source files the team reviews, tests, and promotes.',
    capabilityGroups: [
      {
        title: 'Capabilities',
        items: [
          'Declarative configuration files for identity resources',
          "Environment-specific values supplied separately from the configuration",
          "Version control, review, and rollback through the application's existing workflow",
          'Promotion of changes across dev, staging, and production tenants',
          'Drift detection between configuration files and the live tenant',
        ],
      },
    ],
  },
  {
    id: 'resilience',
    label: 'Resilience',
    title: 'Resilience and Multi-Region Deployment',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="12" r="9" />
        <path d="M3.6 9h16.8M3.6 15h16.8" />
        <path d="M12 3a14.5 14.5 0 0 0 0 18M12 3a14.5 14.5 0 0 1 0 18" />
      </svg>
    ),
    why: 'The identity product is on the critical path for every sign-in. If it is down, no one gets in. The deployment shape needs to match the availability you have promised your users. The right pattern depends on your reliability target, latency budget, and where your users are.',
    capabilityGroups: [
      {
        title: 'Deployment patterns',
        items: [
          'Single-region: simplest deployment, fits regional audiences and modest availability targets',
          'Active-passive: replicates user data to a second region that takes over on failure',
          'Active-active: runs every region live, routing each user to the nearest healthy region for lowest latency and smallest blast radius',
          'Regional sharding: pins specific users to specific regions, often to satisfy data-residency rules',
        ],
      },
      {
        title: 'Capabilities',
        items: [
          'Single-region deployment',
          'Active-passive deployment with failover',
          'Active-active deployment with regional routing',
          'Regional sharding pinned to data residency',
          'User-data replication across regions',
          'Health-checked routing and failover',
          'Independent regional configuration and secrets',
        ],
      },
    ],
  },
  {
    id: 'monitoring',
    label: 'Activity Monitoring',
    title: 'Activity Monitoring and Audit',
    icon: (
      <svg viewBox="0 0 24 24">
        <path d="M3 3v18h18" />
        <path d="m7 16 4-8 4 4 3-6" />
      </svg>
    ),
    why: 'Identity events are some of the highest-signal data in your stack. Every sign-up, sign-in, recovery, and consent change is a row worth keeping. Product teams want funnel and adoption metrics. Security teams want anomalies. Compliance teams want a durable audit log they can hand to a regulator.',
    capabilityGroups: [
      {
        title: 'Capabilities',
        items: [
          'Structured audit log of every identity event with a consistent schema',
          'Built-in dashboards for sign-up funnels, sign-in success rates, and method adoption',
          'Per-user activity timelines for support investigation',
          'Stream events to external analytics, data warehouse, or SIEM platforms',
          'Anomaly detection: failed sign-in spikes, brute force, impossible travel',
          'Real-time alerts for security-relevant events',
          'Export audit logs for compliance reporting',
        ],
      },
    ],
  },
  {
    id: 'connect-to-systems',
    label: 'Connect to Systems',
    title: 'Connect to Other Systems',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="18" cy="5" r="2" />
        <circle cx="6" cy="12" r="2" />
        <circle cx="18" cy="19" r="2" />
        <path d="m8 11.5 8.1-5M8 12.5l8.1 5.1" />
      </svg>
    ),
    why: 'Identity is not an isolated system; it belongs to the business. New sign-ups should land in your CRM; password resets should trigger security event logs; failed sign-ins should surface in your monitoring. The identity product is the natural place to emit these signals because it sees every identity event.',
    capabilityGroups: [
      {
        title: 'Capabilities',
        items: [
          'Emit identity events (sign-up, password change, and so on) to subscribers',
          'Push new-user data to a CRM or marketing tool on sign-up',
          'Send SMS and email notifications through providers you choose',
          'Call out to your own systems mid-journey for validation or data lookup',
          'Run checks before or after key identity actions',
        ],
      },
    ],
  },
];

export function B2COperationsCards() {
  return (
    <Box sx={approachCardsSx}>
      {operationsDetails.map((item) => (
        <UseCaseBuildingBlockPanel
          key={item.id}
          icon={item.icon}
          title={item.title}
          why={item.why}
          capabilityGroups={item.capabilityGroups}
        />
      ))}
    </Box>
  );
}

interface SolutionArchitectureOption {
  id: string;
  label: string;
  description: string;
  graphic: {
    left: string;
    center: string;
    right: string;
    notes: string[];
  };
}

interface SolutionArchitectureStage {
  id: string;
  title: string;
  question: string;
  description: string;
  options: SolutionArchitectureOption[];
}

const solutionArchitectureStages: SolutionArchitectureStage[] = [
  {
    id: 'integration',
    title: 'Choose an integration approach',
    question: 'Where should identity screens live, and who should drive the journey?',
    description: 'Decide who owns the screens and who drives the journey.',
    options: [
      {
        id: 'redirect',
        label: 'Redirect-based',
        description: 'ThunderID hosts the screens and controls the identity journey.',
        graphic: {
          left: 'Your app',
          center: 'Hosted ThunderID journey',
          right: 'Signed-in user',
          notes: ['Sign-in, sign-up, recovery, consent', 'Tokens return to the app', 'Fastest secure path'],
        },
      },
      {
        id: 'app-native',
        label: 'App-native',
        description: 'Your application renders screens while ThunderID controls journey policy.',
        graphic: {
          left: 'App screens',
          center: 'ThunderID journey state',
          right: 'Next step or tokens',
          notes: ['Custom UI', 'Server-controlled branching', 'SDK-driven flow calls'],
        },
      },
      {
        id: 'direct-api',
        label: 'Direct API',
        description: 'Your application owns the screens and composes identity primitives.',
        graphic: {
          left: 'App orchestration',
          center: 'Identity APIs',
          right: 'App-managed outcome',
          notes: ['Maximum control', 'No guided journey', 'More app-side responsibility'],
        },
      },
    ],
  },
  {
    id: 'identity-data',
    title: 'Choose identity sources and data',
    question: 'Where do users come from, and which system owns the user record?',
    description: 'Decide where consumer identities come from and where user records live.',
    options: [
      {
        id: 'managed-directory',
        label: 'ThunderID user store',
        description: 'ThunderID owns the canonical consumer user record.',
        graphic: {
          left: 'Consumers',
          center: 'ThunderID user store',
          right: 'Application profile',
          notes: ['Directly managed users', 'Recovery and profile features', 'Simple operating model'],
        },
      },
      {
        id: 'federation',
        label: 'Federation',
        description: 'Users bring identities from social or enterprise providers.',
        graphic: {
          left: 'External IdPs',
          center: 'Federated sign-in',
          right: 'Linked user',
          notes: ['Social and enterprise OIDC', 'Just-in-time provisioning', 'Home-realm discovery'],
        },
      },
      {
        id: 'mixed',
        label: 'Mixed model',
        description: 'Some users live in ThunderID while other users come from external sources.',
        graphic: {
          left: 'Local and external users',
          center: 'Account linking',
          right: 'One customer identity',
          notes: ['Migration-friendly', 'Segment-specific sources', 'Flexible source of truth'],
        },
      },
    ],
  },
  {
    id: 'tokens-apis',
    title: 'Design tokens, sessions, and APIs',
    question: 'How should the signed-in user be represented to the app and APIs?',
    description: 'Decide what the app receives after sign-in and how APIs trust it.',
    options: [
      {
        id: 'stateless',
        label: 'Stateless tokens',
        description: 'Use short-lived access tokens backed by refresh tokens.',
        graphic: {
          left: 'Application',
          center: 'JWT access token',
          right: 'Protected APIs',
          notes: ['Scales easily', 'Signature validation', 'Best-effort revocation'],
        },
      },
      {
        id: 'revocable',
        label: 'Server-backed sessions',
        description: 'Back refresh and session behavior with revocable server-side records.',
        graphic: {
          left: 'Application session',
          center: 'ThunderID session record',
          right: 'Revocable access',
          notes: ['Sign out everywhere', 'Fast revocation', 'Session lookup'],
        },
      },
      {
        id: 'api-protection',
        label: 'API protection',
        description: 'Shape scopes, audiences, and resource servers around the APIs the app calls.',
        graphic: {
          left: 'Access token',
          center: 'API gateway or middleware',
          right: 'Resource server',
          notes: ['Scopes and permissions', 'Audience validation', 'Shared policy boundary'],
        },
      },
    ],
  },
  {
    id: 'operations',
    title: 'Plan operations and integrations',
    question: 'How should identity configuration run, change, and emit signals?',
    description: 'Decide how identity configuration runs, scales, and connects to your stack.',
    options: [
      {
        id: 'identity-as-code',
        label: 'Identity-as-Code',
        description: 'Keep identity resources in versioned configuration and promote them across environments.',
        graphic: {
          left: 'Configuration files',
          center: 'Deployment pipeline',
          right: 'ThunderID tenants',
          notes: ['Review and rollback', 'Environment values', 'Drift detection'],
        },
      },
      {
        id: 'resilience',
        label: 'Resilience',
        description: 'Choose a deployment shape that matches availability, latency, and residency needs.',
        graphic: {
          left: 'Regional traffic',
          center: 'Healthy region routing',
          right: 'Available sign-in',
          notes: ['Single-region', 'Active-passive or active-active', 'Regional sharding'],
        },
      },
      {
        id: 'events',
        label: 'Audit and events',
        description: 'Send identity activity to dashboards, audit storage, and business systems.',
        graphic: {
          left: 'Identity events',
          center: 'Audit and event stream',
          right: 'Analytics, SIEM, CRM',
          notes: ['Structured audit log', 'Real-time alerts', 'Webhooks and enrichment'],
        },
      },
    ],
  },
];

const solutionCrossCuttingChoices = [
  'Application type',
  'Branding',
  'Session model',
  'Data residency',
  'Token lifetimes',
];

export function B2CSolutionArchitectureMap() {
  const [activeStageId, setActiveStageId] = React.useState(solutionArchitectureStages[0].id);
  const [selectedOptions, setSelectedOptions] = React.useState<Record<string, string>>(
    Object.fromEntries(solutionArchitectureStages.map((stage) => [stage.id, stage.options[0].id])),
  );

  const activeStage =
    solutionArchitectureStages.find((stage) => stage.id === activeStageId) ?? solutionArchitectureStages[0];
  const activeOption =
    activeStage.options.find((option) => option.id === selectedOptions[activeStage.id]) ?? activeStage.options[0];

  const selectOption = (stageId: string, optionId: string) => {
    setSelectedOptions((current) => ({
      ...current,
      [stageId]: optionId,
    }));
  };

  const graphicSx = {
    ...solutionMapGraphicBaseSx,
    background: graphicBackgrounds[activeStage.id] ?? graphicBackgrounds.integration,
  };

  return (
    <Box component="section" sx={solutionMapSx} aria-label="B2C solution architecture decision map">
      <Box sx={solutionMapRailSx} aria-hidden />
      <Box sx={solutionMapStagesSx} role="tablist" aria-label="Solution decision sections">
        {solutionArchitectureStages.map((stage, index) => {
          const isActive = activeStage.id === stage.id;
          return (
            <Box
              key={stage.id}
              component="button"
              type="button"
              role="tab"
              aria-selected={isActive}
              sx={solutionMapStageSx}
              onClick={() => setActiveStageId(stage.id)}
            >
              <Box component="span" sx={isActive ? solutionMapIndexActiveSx : solutionMapIndexSx} aria-hidden>
                {index + 1}
              </Box>
              <Box component="span" className="uc-sm-content" sx={solutionMapContentSx}>
                <Box component="span" sx={solutionMapContentTitleSx}>{stage.title}</Box>
                <Box component="span" sx={solutionMapContentDescSx}>{stage.description}</Box>
                <Box component="span" sx={solutionMapSelectionSx}>
                  {stage.options.find((option) => option.id === selectedOptions[stage.id])?.label ?? stage.options[0].label}
                </Box>
              </Box>
            </Box>
          );
        })}
      </Box>
      <Box component="article" sx={solutionMapDetailSx} role="tabpanel">
        <div>
          <Box component="span" sx={solutionChooserEyebrowSx}>Decision {solutionArchitectureStages.indexOf(activeStage) + 1}</Box>
          <h3>{activeStage.question}</h3>
          <Box sx={solutionMapOptionsSx} role="group" aria-label={activeStage.question}>
            {activeStage.options.map((option) => {
              const isActive = activeOption.id === option.id;
              return (
                <Box
                  key={option.id}
                  component="button"
                  type="button"
                  sx={isActive ? solutionMapOptionActiveSx : solutionMapOptionSx}
                  onClick={() => selectOption(activeStage.id, option.id)}
                >
                  <strong>{option.label}</strong>
                  <span>{option.description}</span>
                </Box>
              );
            })}
          </Box>
        </div>
        <Box sx={graphicSx} aria-label={`${activeOption.label} architecture graphic`}>
          <Box sx={solutionMapGraphicFlowSx}>
            <Box sx={solutionMapGraphicNodeSx}>{activeOption.graphic.left}</Box>
            <Box component="span" sx={solutionMapGraphicArrowSx} aria-hidden>
              →
            </Box>
            <Box sx={solutionMapGraphicNodePrimarySx}>
              {activeOption.graphic.center}
            </Box>
            <Box component="span" sx={solutionMapGraphicArrowSx} aria-hidden>
              →
            </Box>
            <Box sx={solutionMapGraphicNodeSx}>{activeOption.graphic.right}</Box>
          </Box>
          <ul>
            {activeOption.graphic.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
        </Box>
      </Box>
      <Box component="aside" sx={solutionMapCrossCuttingSx} aria-label="Cross-cutting choices">
        <div>
          <h3>Apply cross-cutting choices</h3>
          <p>These choices affect every stage of the solution.</p>
        </div>
        <ul>
          {solutionCrossCuttingChoices.map((choice) => (
            <li key={choice}>{choice}</li>
          ))}
        </ul>
      </Box>
    </Box>
  );
}

export function B2CIdentitySourcesDataGraphic() {
  return (
    <Box component="section" sx={identitySourcesDiagramSx} aria-label="Identity sources and data overview">
      <Box sx={identitySourcesHeroSx}>
        <Box sx={identitySourcesHeroIconSx}>
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <circle cx="12" cy="8" r="4" />
            <path d="M4 20c0-4 3.6-7 8-7s8 3 8 7" />
          </svg>
        </Box>
        <strong>User identity</strong>
      </Box>

      <Box sx={identitySourcesForkSx} aria-hidden="true">
        <svg viewBox="0 0 200 32" preserveAspectRatio="none">
          <line x1="100" y1="0" x2="100" y2="16" />
          <line x1="30" y1="16" x2="170" y2="16" />
          <line x1="30" y1="16" x2="30" y2="32" />
          <line x1="170" y1="16" x2="170" y2="32" />
        </svg>
      </Box>

      <Box sx={identitySourcesQuestionsSx}>
        <Box sx={identitySourcesQuestionSx}>
          <Box sx={identitySourcesQuestionHeadSx}>
            <Box component="span" sx={identitySourcesEyebrowSx}>Question 1</Box>
            <h3><a href="#identity-federation">How does identity enter the app?</a></h3>
            <p>How consumer identities arrive at your application.</p>
          </Box>
          <Box sx={identitySourcesItemGroupSx}>
            <h4>Identity providers</h4>
            <Box component="ul" sx={identitySourcesItemsSx}>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 3c-2.5 3-4 5.6-4 9s1.5 6 4 9M12 3c2.5 3 4 5.6 4 9s-1.5 6-4 9M3 12h18" />
                  </svg>
                </Box>
                <span>
                  <strong>Social sign-in</strong>
                  <span>Google, GitHub, and other consumer providers</span>
                </span>
              </li>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <rect x="3" y="6" width="18" height="13" rx="2" />
                    <path d="M8 6V4h8v2" />
                  </svg>
                </Box>
                <span>
                  <strong>Enterprise OIDC</strong>
                  <span>Connect enterprise identity providers</span>
                </span>
              </li>
            </Box>
          </Box>

          <Box sx={identitySourcesItemGroupSx}>
            <h4>Federation decisions</h4>
            <Box component="ul" sx={identitySourcesItemsSx}>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <circle cx="12" cy="12" r="9" />
                    <path d="M12 8v8M8 12h8" />
                  </svg>
                </Box>
                <span>
                  <strong>Provisioning model</strong>
                  <span>JIT on first sign-in, or invitation-only onboarding</span>
                </span>
              </li>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M8 12h8" />
                    <circle cx="6" cy="12" r="2" />
                    <circle cx="18" cy="12" r="2" />
                  </svg>
                </Box>
                <span>
                  <strong>Account linking policy</strong>
                  <span>Verified email, explicit user action, or both</span>
                </span>
              </li>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M8 7H5a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3" />
                    <path d="M8 7l2.5-3h3L16 7" />
                  </svg>
                </Box>
                <span>
                  <strong>Single logout</strong>
                  <span>Sign the user out of your app and connected provider</span>
                </span>
              </li>
              <li>
                <Box component="span" sx={identitySourcesItemIconSx}>
                  <svg viewBox="0 0 24 24" aria-hidden="true">
                    <path d="M3 9l9-6 9 6v10a2 2 0 0 1-2 2h-3" />
                    <path d="M12 12h6M15 9l3 3-3 3" />
                  </svg>
                </Box>
                <span>
                  <strong>Home-realm discovery</strong>
                  <span>Route users to the right provider by email domain</span>
                </span>
              </li>
            </Box>
          </Box>
        </Box>

        <Box sx={identitySourcesQuestionSx}>
          <Box sx={identitySourcesQuestionHeadSx}>
            <Box component="span" sx={identitySourcesEyebrowSx}>Question 2</Box>
            <h3><a href="#user-stores">Where is it stored?</a></h3>
            <p>Which system owns the canonical user record.</p>
          </Box>
          <Box component="ul" sx={identitySourcesItemsSx}>
            <li>
              <Box component="span" sx={identitySourcesItemIconSx}>
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <ellipse cx="12" cy="7" rx="9" ry="3" />
                  <path d="M3 7v5c0 1.66 4.03 3 9 3s9-1.34 9-3V7M3 12v5c0 1.66 4.03 3 9 3s9-1.34 9-3v-5" />
                </svg>
              </Box>
              <span>
                <strong>Product-managed directory</strong>
                <span>ThunderID owns the canonical user record</span>
              </span>
            </li>
            <li>
              <Box component="span" sx={identitySourcesItemIconSx}>
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <circle cx="12" cy="12" r="9" />
                  <path d="M9 12h6M12 9l3 3-3 3" />
                </svg>
              </Box>
              <span>
                <strong>Federated-only</strong>
                <span>No local record; identity stays with the provider</span>
              </span>
            </li>
            <li>
              <Box component="span" sx={identitySourcesItemIconSx}>
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <rect x="2" y="3" width="8" height="5" rx="1" />
                  <rect x="14" y="3" width="8" height="5" rx="1" />
                  <rect x="2" y="16" width="8" height="5" rx="1" />
                  <path d="M6 8v4h12V8M18 16v-4" />
                </svg>
              </Box>
              <span>
                <strong>External directory</strong>
                <span>LDAP or custom backing store you control</span>
              </span>
            </li>
            <li>
              <Box component="span" sx={identitySourcesItemIconSx}>
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <rect x="3" y="9" width="7" height="6" rx="1" />
                  <rect x="14" y="9" width="7" height="6" rx="1" />
                  <path d="M10 12h4" />
                </svg>
              </Box>
              <span>
                <strong>Mixed</strong>
                <span>Some managed, others federated</span>
              </span>
            </li>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

export function B2CSolutionPatternsExplorer() {
  const [uiOwner, setUiOwner] = React.useState<'thunderid' | 'app'>('thunderid');
  const [journeyOwner, setJourneyOwner] = React.useState<'thunderid' | 'app'>('thunderid');

  const selectedPatternId =
    uiOwner === 'app' && journeyOwner === 'app'
      ? 'direct-api'
      : uiOwner === 'app'
        ? 'app-native'
        : 'redirect-based';
  const selectedPattern =
    solutionPatternDetails.find((p) => p.id === selectedPatternId) ?? solutionPatternDetails[0];

  const selectPattern = (patternId: string) => {
    if (patternId === 'direct-api') {
      setUiOwner('app');
      setJourneyOwner('app');
      return;
    }

    if (patternId === 'app-native') {
      setUiOwner('app');
      setJourneyOwner('thunderid');
      return;
    }

    setUiOwner('thunderid');
    setJourneyOwner('thunderid');
  };

  const selectUiOwner = (owner: 'thunderid' | 'app') => {
    setUiOwner(owner);
    if (owner === 'thunderid') setJourneyOwner('thunderid');
  };

  const isFirstRender = React.useRef(true);
  React.useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    if (typeof window !== 'undefined') {
      window.dispatchEvent(
        new CustomEvent('thunder:pattern-selected', {
          detail: { id: selectedPatternId, label: selectedPattern.label },
        })
      );
    }
  }, [selectedPatternId, selectedPattern.label]);

  return (
    <Box component="section" sx={solutionChooserSx} aria-label="B2C solution pattern chooser">
      <Box sx={solutionChooserQuestionsSx} aria-label="Architecture decisions">
        <Box sx={solutionChooserQuestionSx}>
          <div>
            <Box component="span" sx={solutionChooserEyebrowSx}>Decision 1</Box>
            <h3>Who owns the identity screens?</h3>
            <p>Choose where users see sign-in, sign-up, recovery, and consent screens.</p>
          </div>
          <Box sx={solutionChooserOptionsSx} role="group" aria-label="Choose who owns the identity screens">
            <Box
              component="button"
              type="button"
              sx={uiOwner === 'thunderid' ? solutionChooserOptionActiveSx : solutionChooserOptionSx}
              onClick={() => selectUiOwner('thunderid')}
            >
              ThunderID
            </Box>
            <Box
              component="button"
              type="button"
              sx={uiOwner === 'app' ? solutionChooserOptionActiveSx : solutionChooserOptionSx}
              onClick={() => selectUiOwner('app')}
            >
              Your application
            </Box>
          </Box>
        </Box>

        <Box sx={solutionChooserQuestionSx}>
          <div>
            <Box component="span" sx={solutionChooserEyebrowSx}>Decision 2</Box>
            <h3>Who owns the identity journey?</h3>
            <p>Choose who decides the next step, applies policy, and handles branching.</p>
          </div>
          <Box sx={solutionChooserOptionsSx} role="group" aria-label="Choose who owns the identity journey">
            <Box
              component="button"
              type="button"
              sx={journeyOwner === 'thunderid' ? solutionChooserOptionActiveSx : solutionChooserOptionSx}
              onClick={() => setJourneyOwner('thunderid')}
            >
              ThunderID
            </Box>
            <Box
              component="button"
              type="button"
              sx={journeyOwner === 'app' ? solutionChooserOptionActiveSx : solutionChooserOptionSx}
              disabled={uiOwner === 'thunderid'}
              onClick={() => setJourneyOwner('app')}
            >
              Your application
            </Box>
          </Box>
        </Box>
      </Box>

      <Box sx={solutionChooserPatternsSx} role="tablist" aria-label="Solution patterns">
        {solutionPatternDetails.map((pattern) => (
          <button
            key={pattern.id}
            type="button"
            role="tab"
            aria-selected={selectedPattern.id === pattern.id}
            className={`uc-building-block-node${selectedPattern.id === pattern.id ? ' uc-building-block-node--active' : ''}`}
            onClick={() => selectPattern(pattern.id)}
          >
            <span className="uc-building-block-node__icon" aria-hidden>
              {pattern.icon}
            </span>
            {selectedPattern.id === pattern.id && <Box component="span" sx={solutionChooserRecommendedSx}>Recommended</Box>}
            <span className="uc-building-block-node__label">{pattern.label}</span>
          </button>
        ))}
      </Box>

      <article className="uc-building-blocks__panel" style={{marginTop: 0}} role="tabpanel">
        <div className="uc-building-blocks__body">
          <p>{selectedPattern.why}</p>
          <Box component="a" href={`#${selectedPattern.id}`} sx={solutionChooserRecLinkSx}>
            Read the {selectedPattern.title} details
            <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" width={14} height={14}>
              <path d="M5 12h14M12 5l7 7-7 7" />
            </svg>
          </Box>
        </div>
      </article>
    </Box>
  );
}

export function B2CIdentityJourneyRoadmap() {
  return (
    <Box component="nav" sx={roadmapSx} aria-label="B2C identity use case roadmap">
      {roadmapNodes.map((node) => (
        <Box key={node.href} component="a" href={node.href} sx={roadmapNodeSx}>
          <Box component="span" className="uc-b2c-roadmap__icon" sx={roadmapIconSx} aria-hidden>
            {node.icon}
          </Box>
          <Box component="span" sx={roadmapLabelSx}>{node.label}</Box>
        </Box>
      ))}
    </Box>
  );
}

export function B2CSolutionPatternsRoadmap() {
  return (
    <Box component="nav" sx={roadmapSx} aria-label="B2C solution pattern roadmap">
      {solutionPatternNodes.map((node) => (
        <Box key={node.href} component="a" href={node.href} sx={roadmapNodeSx}>
          <Box component="span" className="uc-b2c-roadmap__icon" sx={roadmapIconSx} aria-hidden>
            {node.icon}
          </Box>
          <Box component="span" sx={roadmapLabelSx}>{node.label}</Box>
        </Box>
      ))}
    </Box>
  );
}

interface ArchDecisionCard {
  id: 'integration' | 'identity-sources' | 'tokens-and-apis' | 'operations';
  title: string;
  question: string;
  href: string;
  icon: React.ReactNode;
}

const b2cArchDecisions: ArchDecisionCard[] = [
  {
    id: 'integration',
    title: 'Integration Pattern',
    question: 'Where do identity screens live, and who controls the journey?',
    href: '../integration-patterns',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="12" cy="18" r="3" />
        <circle cx="6" cy="6" r="3" />
        <circle cx="18" cy="6" r="3" />
        <path d="M18 9v2c0 .6-.4 1-1 1H7c-.6 0-1-.4-1-1V9" />
        <path d="M12 12v3" />
      </svg>
    ),
  },
  {
    id: 'identity-sources',
    title: 'Identity Sources',
    question: 'Where do identities come from, and which system owns the record?',
    href: '../identity-sources',
    icon: (
      <svg viewBox="0 0 24 24">
        <ellipse cx="12" cy="5" rx="9" ry="3" />
        <path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5" />
        <path d="M3 12c0 1.66 4.03 3 9 3s9-1.34 9-3" />
      </svg>
    ),
  },
  {
    id: 'tokens-and-apis',
    title: 'Tokens & APIs',
    question: 'How are post-sign-in credentials shaped, and how do your APIs validate them?',
    href: '../tokens-and-apis',
    icon: (
      <svg viewBox="0 0 24 24">
        <circle cx="7.5" cy="15.5" r="5.5" />
        <path d="m21 2-9.6 9.6" />
        <path d="m15.5 7.5 3 3L22 7l-3-3" />
      </svg>
    ),
  },
  {
    id: 'operations',
    title: 'Run & Observe',
    question: 'How do you configure, deploy, monitor, and connect the identity system?',
    href: '../operations',
    icon: (
      <svg viewBox="0 0 24 24">
        <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
      </svg>
    ),
  },
];

function GlassCard({
  href = undefined,
  sx: extraSx = {},
  children = undefined,
}: {
  href?: string;
  sx?: object;
  children?: React.ReactNode;
}) {
  const cardSx = { ...glassCardSx, ...extraSx };
  if (href) return <Box component={Link} to={href} sx={cardSx}>{children}</Box>;
  return <Box sx={cardSx}>{children}</Box>;
}

export function B2CNextSteps({ href = './try-it-out' }: { href?: string } = {}) {
  const [patternLabel, setPatternLabel] = React.useState<string | null>(null);

  React.useEffect(() => {
    const handle = (e: Event) => {
      setPatternLabel((e as CustomEvent<{ label: string }>).detail.label);
    };
    window.addEventListener('thunder:pattern-selected', handle);
    return () => window.removeEventListener('thunder:pattern-selected', handle);
  }, []);

  return (
    <Box sx={nextStepsSx}>
      <GlassCard href={href} sx={nextStepsTrySx}>
        <Box sx={nextStepsTryEyebrowSx}>Try It Out</Box>
        <Box sx={nextStepsTryTitleSx}>
          {patternLabel ? `See ${patternLabel} working in practice` : 'See your pattern working in practice'}
        </Box>
        <Box component="p" sx={nextStepsTryDescSx}>
          Walk through a working B2C setup and see how your selected integration pattern behaves end to end.
        </Box>
        <Box component="span" sx={nextStepsTryBtnSx}>Start the walkthrough &#8594;</Box>
      </GlassCard>
    </Box>
  );
}

export function B2CArchitectureDecisions({
  currentDecision,
  prioritizeIntegration = false,
}: {
  currentDecision?: ArchDecisionCard['id'];
  prioritizeIntegration?: boolean;
} = {}) {
  const cards = currentDecision
    ? b2cArchDecisions.filter((d) => d.id !== currentDecision)
    : b2cArchDecisions;

  if (prioritizeIntegration) {
    const integration = b2cArchDecisions.find((d) => d.id === 'integration') ?? b2cArchDecisions[0];
    const supporting = b2cArchDecisions.filter((d) => d.id !== 'integration');
    return (
      <Box sx={archDecisionsPrioritizedSx}>
        <Box sx={archDecisionsPrimarySx}>
          <Box component="span" sx={archDecisionsStepSx}>Start here</Box>
          <GlassCard href={integration.href} sx={archDecisionCardPrimarySx}>
            <Box sx={iconContainerSx}>{integration.icon}</Box>
            <Box sx={archDecisionCardBodyPrimarySx}>
              <Box sx={archDecisionCardTitleSx}>{integration.title}</Box>
              <Box component="p" sx={archDecisionCardQuestionSx}>{integration.question}</Box>
              <Box component="span" sx={archDecisionCardCtaPrimarySx}>Choose an integration pattern &#8594;</Box>
            </Box>
          </GlassCard>
        </Box>
        <Box sx={archDecisionsSupportingSx}>
          <Box component="span" sx={archDecisionsStepSx}>Supporting decisions</Box>
          <Box sx={archDecisionsSupportingGridSx}>
            {supporting.map((d) => (
              <GlassCard key={d.id} href={d.href} sx={archDecisionCardSupportingSx}>
                <Box sx={supportingIconContainerSx}>{d.icon}</Box>
                <Box sx={archDecisionCardBodySupportingSx}>
                  <Box sx={archDecisionCardTitleSupportingSx}>{d.title}</Box>
                  <Box component="p" sx={archDecisionCardQuestionSupportingSx}>{d.question}</Box>
                  <Box component="span" sx={archDecisionCardCtaSupportingSx}>Explore &#8594;</Box>
                </Box>
              </GlassCard>
            ))}
          </Box>
        </Box>
      </Box>
    );
  }

  return (
    <Box sx={archDecisionsSx}>
      <Box sx={archDecisionsGridSx}>
        {cards.map((d) => (
          <GlassCard key={d.id} href={d.href} sx={archDecisionCardBaseSx}>
            <Box sx={iconContainerSx}>{d.icon}</Box>
            <Box sx={archDecisionCardTitleSx}>{d.title}</Box>
            <Box component="p" sx={archDecisionCardQuestionSx}>{d.question}</Box>
            <Box component="span" sx={archDecisionCardCtaSx}>Explore &#8594;</Box>
          </GlassCard>
        ))}
      </Box>
    </Box>
  );
}
