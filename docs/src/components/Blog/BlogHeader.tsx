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

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import ProductName from '@site/src/components/ProductName';
import useIsDarkMode from '../../hooks/useIsDarkMode';

export default function BlogHeader(): JSX.Element {
  const isDark = useIsDarkMode();
  const eyebrowColor = isDark ? '#8bf9fa' : '#1a6fe8';
  return (
    <Box sx={{maxWidth: 1200, width: '100%', mx: 'auto', px: {xs: 2, sm: 4}, pt: {xs: 5, md: 7}, pb: 1}}>
      <Box
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 1,
          mb: 2,
          fontFamily: 'monospace',
          fontSize: '10.5px',
          fontWeight: 600,
          letterSpacing: '0.18em',
          textTransform: 'uppercase',
          color: eyebrowColor,
        }}
      >
        <Box component="span" sx={{width: 5, height: 5, borderRadius: '50%', bgcolor: eyebrowColor, boxShadow: `0 0 10px ${eyebrowColor}`}} />
        The <ProductName /> blog
      </Box>

      <Typography
        variant="h1"
        sx={{
          fontSize: {xs: '2.25rem', sm: '3rem', md: '3.75rem'},
          fontWeight: 700,
          lineHeight: 1.04,
          letterSpacing: '-0.035em',
          maxWidth: '16ch',
          color: 'text.primary',
          mb: 2,
        }}
      >
        Notes from{' '}
        <Box
          component="span"
          sx={{
            background: 'linear-gradient(92deg,#8bf9fa 0%,#3688ff 55%)',
            WebkitBackgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
            backgroundClip: 'text',
          }}
        >
          building auth
        </Box>{' '}
        in the open
      </Typography>

      <Typography sx={{fontSize: '17px', lineHeight: 1.65, color: 'text.secondary', maxWidth: 560}}>
        Deep dives, release notes, and engineering write-ups from the team building an open-source identity stack for
        humans, AI agents, and workloads.
      </Typography>
    </Box>
  );
}
