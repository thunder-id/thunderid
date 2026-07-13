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

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Button, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import ProductName from '@site/src/components/ProductName';

export default function EcosystemCTA(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const config = siteConfig.customFields?.product as DocusaurusProductConfig;
  const discussionUrl = `${config.project.source.github.discussionsUrl}/new?category=ideas&title=${encodeURIComponent('[Request SDK]')}`;

  return (
    <Box sx={{maxWidth: 1200, mx: 'auto', px: {xs: 2, sm: 4}, pb: {xs: 8, md: 10}}}>
      <Box
        sx={{
          position: 'relative',
          overflow: 'hidden',
          borderRadius: '20px',
          border: '1px solid rgba(54,136,255,0.2)',
          bgcolor: 'rgba(54,136,255,0.05)',
          p: {xs: 4, md: 6},
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 3,
        }}
      >
        <Box
          sx={{
            position: 'absolute',
            top: -80,
            right: -40,
            width: 340,
            height: 340,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(54,136,255,0.14) 0%, transparent 65%)',
            pointerEvents: 'none',
          }}
        />
        <Box sx={{position: 'relative', maxWidth: 560}}>
          <Typography
            component="h2"
            sx={{fontSize: {xs: '22px', md: '30px'}, fontWeight: 700, letterSpacing: '-0.03em', color: 'text.primary', mb: 1.5}}
          >
            Don&rsquo;t see your framework?
          </Typography>
          <Typography sx={{fontSize: '14.5px', lineHeight: 1.65, color: 'text.secondary'}}>
            Start a discussion and it can be brought in as an official out-of-the-box SDK or a community integration.
            Every <ProductName /> SDK is built on the same framework-agnostic core, adding a new one is straightforward.
          </Typography>
        </Box>
        <Button
          variant="contained"
          color="primary"
          href={discussionUrl}
          target="_blank"
          rel="noopener noreferrer"
          size="large"
          endIcon={<ArrowRight size={14} strokeWidth={2.4} />}
        >
          Start a discussion
        </Button>
      </Box>
    </Box>
  );
}
