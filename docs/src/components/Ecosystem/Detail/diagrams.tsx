// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX, ReactNode} from 'react';
import {useCyan, useInk} from './theme';
import ProductName from '../../ProductName';

/* ─── Primitives ──────────────────────────────────────────────────────────── */

function Code({children}: {children: ReactNode}): JSX.Element {
  const cyan = useCyan();
  return (
    <Box
      component="code"
      sx={{
        fontFamily: 'monospace',
        fontSize: '0.92em',
        color: cyan,
        bgcolor: `color-mix(in srgb, ${cyan} 8%, transparent)`,
        borderRadius: '4px',
        px: '5px',
        py: '1px',
      }}
    >
      {children}
    </Box>
  );
}

function Node({
  area,
  eyebrow,
  title,
  accent,
  highlight = false,
  children,
}: {
  area: string;
  eyebrow: string;
  title: ReactNode;
  accent: string;
  highlight?: boolean;
  children: ReactNode;
}): JSX.Element {
  const ink = useInk();
  return (
    <Box
      sx={{
        gridArea: area,
        px: 2,
        py: 1.75,
        border: '1px solid',
        borderColor: highlight ? `color-mix(in srgb, ${accent} 55%, transparent)` : ink(0.1, 0.1),
        bgcolor: highlight ? `color-mix(in srgb, ${accent} 8%, transparent)` : ink(0.02, 0.03),
        borderRadius: '10px',
      }}
    >
      <Typography
        sx={{
          fontFamily: 'monospace',
          fontSize: '9.5px',
          letterSpacing: '0.12em',
          textTransform: 'uppercase',
          color: highlight ? accent : ink(0.45, 0.45),
          mb: 0.75,
        }}
      >
        {eyebrow}
      </Typography>
      <Typography sx={{fontSize: '14px', fontWeight: 600, color: ink(0.85, 0.92), mb: 0.5}}>{title}</Typography>
      <Typography sx={{fontSize: '12.5px', lineHeight: 1.6, color: ink(0.55, 0.55)}}>{children}</Typography>
    </Box>
  );
}

type Direction = 'right' | 'left' | 'down' | 'up';

/** A straight arrow that fills its box along its own axis. */
function Arrow({
  direction,
  colour,
  dashed = false,
  both = false,
}: {
  direction: Direction;
  colour: string;
  dashed?: boolean;
  both?: boolean;
}): JSX.Element {
  const vertical = direction === 'down' || direction === 'up';
  const head = (towards: Direction): JSX.Element => (
    <Box
      sx={{
        width: 0,
        height: 0,
        flexShrink: 0,
        borderStyle: 'solid',
        borderColor: 'transparent',
        ...(towards === 'right' && {borderWidth: '4px 0 4px 7px', borderLeftColor: colour}),
        ...(towards === 'left' && {borderWidth: '4px 7px 4px 0', borderRightColor: colour}),
        ...(towards === 'down' && {borderWidth: '7px 4px 0 4px', borderTopColor: colour}),
        ...(towards === 'up' && {borderWidth: '0 4px 7px 4px', borderBottomColor: colour}),
      }}
    />
  );
  const start = direction === 'right' ? 'left' : direction === 'down' ? 'up' : direction;
  const end = direction === 'left' ? 'right' : direction === 'up' ? 'down' : direction;

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: vertical ? 'column' : 'row',
        alignItems: 'center',
        ...(vertical ? {height: 40} : {width: '100%'}),
      }}
    >
      {(both || direction === 'left' || direction === 'up') && head(start)}
      <Box
        sx={{
          flex: 1,
          ...(vertical
            ? {borderLeft: `1.5px ${dashed ? 'dashed' : 'solid'} ${colour}`}
            : {borderTop: `1.5px ${dashed ? 'dashed' : 'solid'} ${colour}`}),
        }}
      />
      {(both || direction === 'right' || direction === 'down') && head(end)}
    </Box>
  );
}

function ArrowLabel({children}: {children: ReactNode}): JSX.Element {
  const ink = useInk();
  return (
    <Typography sx={{fontFamily: 'monospace', fontSize: '10.5px', color: ink(0.5, 0.5), textAlign: 'center'}}>
      {children}
    </Typography>
  );
}

/* ─── Backend for Frontend ────────────────────────────────────────────────── */

/**
 * The React app talks only to the BFF; the BFF talks to the identity provider.
 * Three columns on wide screens, one on narrow ones, where the arrows between
 * the app and the BFF turn vertical.
 */
export function BackendForFrontendDiagram({accent}: {accent: string}): JSX.Element {
  const ink = useInk();
  const muted = ink(0.35, 0.4);

  return (
    <Box
      role="img"
      aria-label="The React app in the browser sends /api/auth requests with its session cookie to the BFF on your server. The BFF holds the tokens, signs in with the identity provider, and receives back-channel logout from it."
      sx={{
        display: 'grid',
        gridTemplateColumns: {xs: '1fr', md: '1fr 180px 1fr'},
        gridTemplateAreas: {
          xs: '"browser" "link" "server" "down" "idp"',
          md: '"browser link server" ". . down" ". . idp"',
        },
        alignItems: 'center',
        columnGap: 1,
        p: {xs: 2, md: 3},
        border: '1px solid',
        borderColor: ink(0.08, 0.07),
        bgcolor: ink(0.015, 0.02),
        borderRadius: '12px',
      }}
    >
      <Node area="browser" eyebrow="Browser" title="React app" accent={accent}>
        Holds an <Code>HttpOnly</Code> session cookie. No refresh or ID token.
      </Node>

      <Box sx={{gridArea: 'link', display: {xs: 'none', md: 'flex'}, flexDirection: 'column', gap: 0.75, px: 0.5}}>
        <ArrowLabel>/api/auth/* + cookie</ArrowLabel>
        <Arrow direction="right" colour={accent} />
        <Arrow direction="left" colour={muted} dashed />
        <ArrowLabel>session, API responses</ArrowLabel>
      </Box>
      <Box sx={{gridArea: 'link', display: {xs: 'flex', md: 'none'}, justifyContent: 'center', gap: 1.5, py: 0.75}}>
        <Box sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
          <Arrow direction="down" colour={accent} />
          <ArrowLabel>/api/auth/* + cookie</ArrowLabel>
        </Box>
        <Box sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
          <Arrow direction="up" colour={muted} dashed />
          <ArrowLabel>session, API responses</ArrowLabel>
        </Box>
      </Box>

      <Node area="server" eyebrow="Your server" title="BFF (Express)" accent={accent} highlight>
        Holds the access, refresh, and ID tokens and the client secret. Adds the access token to API calls.
      </Node>

      <Box sx={{gridArea: 'down', display: 'flex', justifyContent: 'center', alignItems: 'center', gap: 1, py: 0.75}}>
        <Arrow direction="down" colour={muted} both />
        <ArrowLabel>sign-in, tokens, back-channel logout</ArrowLabel>
      </Box>

      <Node area="idp" eyebrow="Identity provider" title={<ProductName />} accent={accent}>
        Tells the BFF, server to server, when the user&apos;s session ends.
      </Node>
    </Box>
  );
}
