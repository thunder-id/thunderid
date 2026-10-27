// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, type JSX} from 'react';
import {useLocation, useNavigate} from 'react-router';
import {useEmbed} from './EmbedContext';

/**
 * Lives inside the router: reports in-app navigation to the host, for its sidebar, and gives the
 * host a way to navigate the console's router.
 */
export default function EmbedRouterBridge(): JSX.Element | null {
  const embed = useEmbed();
  const navigate = useNavigate();
  const {pathname} = useLocation();

  useEffect(() => {
    if (!embed) {
      return undefined;
    }

    embed.setNavigate((path: string) => {
      void navigate(path);
    });

    return () => {
      embed.setNavigate(null);
    };
  }, [embed, navigate]);

  useEffect(() => {
    embed?.ctx.onNavigate(pathname);
  }, [embed, pathname]);

  return null;
}
