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
import type {BlogPostContextValue} from '@docusaurus/plugin-content-blog/client';
import {Box, Button, ListItemIcon, ListItemText, Menu, MenuItem, Typography} from '@wso2/oxygen-ui';
import {Check, Clock, Link2, Share2} from '@wso2/oxygen-ui-icons-react';
import {JSX, MouseEvent, useState} from 'react';
import BlogAvatar from './BlogAvatar';
import {formatDate, formatReadingTime, getBannerImage, getCategory} from './helpers';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import FacebookIcon from '../icons/FacebookIcon';
import LinkedInIcon from '../icons/LinkedInIcon';
import XIcon from '../icons/XIcon';

export default function BlogPostHero({content}: {content: BlogPostContextValue}): JSX.Element {
  const isLight = !useIsDarkMode();
  const {metadata} = content;
  const author = metadata.authors[0];
  const category = getCategory(content);
  const bannerImage = getBannerImage(content);
  const [copied, setCopied] = useState(false);
  const [shareAnchor, setShareAnchor] = useState<HTMLElement | null>(null);

  const getPageUrl = () => (typeof window !== 'undefined' ? window.location.href : metadata.permalink);

  const openShareWindow = (url: string) => {
    window.open(url, '_blank', 'noopener,noreferrer,width=600,height=500');
    setShareAnchor(null);
  };

  const shareToX = () => {
    const params = new URLSearchParams({text: metadata.title, url: getPageUrl()});
    openShareWindow(`https://twitter.com/intent/tweet?${params.toString()}`);
  };

  const shareToLinkedIn = () => {
    const params = new URLSearchParams({url: getPageUrl()});
    openShareWindow(`https://www.linkedin.com/sharing/share-offsite/?${params.toString()}`);
  };

  const shareToFacebook = () => {
    const params = new URLSearchParams({u: getPageUrl()});
    openShareWindow(`https://www.facebook.com/sharer/sharer.php?${params.toString()}`);
  };

  const copyLink = async () => {
    await navigator.clipboard.writeText(getPageUrl());
    setCopied(true);
    setShareAnchor(null);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Box component="header" sx={{pt: {xs: 5, md: 7}, position: 'relative', overflow: 'hidden'}}>
      <Box sx={{maxWidth: 1200, width: '100%', mx: 'auto', px: {xs: 2, sm: 4}}}>
        <Box sx={{display: 'flex', alignItems: 'center', gap: 1, mb: 3.5}}>
          <Box
            component={Link}
            to="/blog"
            sx={{
              fontSize: '13px',
              color: isLight ? 'rgba(0,0,0,0.42)' : 'rgba(255,255,255,0.42)',
              transition: 'color 0.15s',
              '&:hover': {color: 'text.primary'},
            }}
          >
            Blog
          </Box>
          <Typography component="span" sx={{color: isLight ? 'rgba(0,0,0,0.18)' : 'rgba(255,255,255,0.18)', fontSize: '13px'}}>
            /
          </Typography>
          <Typography component="span" sx={{fontSize: '13px', color: isLight ? 'rgba(0,0,0,0.32)' : 'rgba(255,255,255,0.32)'}}>
            {category}
          </Typography>
        </Box>

        <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, mb: 2.5, flexWrap: 'wrap'}}>
          <Box
            component="span"
            sx={{
              fontFamily: 'monospace',
              fontSize: '10px',
              fontWeight: 600,
              textTransform: 'uppercase',
              color: isLight ? '#1856b3' : '#8bf9fa',
              bgcolor: 'rgba(54,136,255,0.12)',
              border: '1px solid rgba(54,136,255,0.28)',
              borderRadius: '6px',
              px: 1.1,
              py: 0.4,
            }}
          >
            {category}
          </Box>
          {typeof metadata.readingTime !== 'undefined' && (
            <Box
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 0.6,
                fontFamily: 'monospace',
                fontSize: '11.5px',
                color: isLight ? 'rgba(0,0,0,0.36)' : 'rgba(255,255,255,0.36)',
              }}
            >
              <Clock size={12} />
              {formatReadingTime(metadata.readingTime)}
            </Box>
          )}
          <Typography
            component="span"
            sx={{fontFamily: 'monospace', fontSize: '11.5px', color: isLight ? 'rgba(0,0,0,0.28)' : 'rgba(255,255,255,0.28)'}}
          >
            {formatDate(metadata.date)}
          </Typography>
        </Box>

        <Typography
          component="h1"
          sx={{
            fontSize: {xs: '30px', sm: '38px', md: '48px'},
            fontWeight: 700,
            lineHeight: 1.1,
            letterSpacing: '-0.03em',
            mb: 2.5,
            color: 'text.primary',
            maxWidth: 640,
            textWrap: 'pretty',
          }}
        >
          {metadata.title}
        </Typography>

        {metadata.description && (
          <Typography sx={{fontSize: '17.5px', lineHeight: 1.7, color: 'text.secondary', mb: 4, maxWidth: 640, textWrap: 'pretty'}}>
            {metadata.description}
          </Typography>
        )}

        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 2,
            py: 2.5,
            borderTop: '1px solid',
            borderBottom: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
            flexWrap: 'wrap',
          }}
        >
          {author && (
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
              <BlogAvatar name={author.name ?? ''} imageURL={author.imageURL} size={42} />
              <Box>
                <Typography sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary'}}>{author.name}</Typography>
                {author.title && (
                  <Typography sx={{fontSize: '12.5px', color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)'}}>
                    {author.title}
                  </Typography>
                )}
              </Box>
            </Box>
          )}

          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <Button
              size="small"
              variant="outlined"
              onClick={(event: MouseEvent<HTMLButtonElement>) => setShareAnchor(event.currentTarget)}
              startIcon={copied ? <Check size={13} /> : <Share2 size={13} />}
              sx={{
                fontSize: '12.5px',
                fontWeight: 500,
                textTransform: 'none',
                borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
                color: isLight ? 'rgba(0,0,0,0.55)' : 'rgba(255,255,255,0.55)',
                '&:hover': {borderColor: isLight ? 'rgba(0,0,0,0.22)' : 'rgba(255,255,255,0.22)', color: 'text.primary'},
              }}
            >
              {copied ? 'Copied' : 'Share'}
            </Button>
            <Menu anchorEl={shareAnchor} open={Boolean(shareAnchor)} onClose={() => setShareAnchor(null)}>
              <MenuItem onClick={shareToX}>
                <ListItemIcon>
                  <XIcon size={16} />
                </ListItemIcon>
                <ListItemText>Share on X</ListItemText>
              </MenuItem>
              <MenuItem onClick={shareToLinkedIn}>
                <ListItemIcon>
                  <LinkedInIcon size={16} />
                </ListItemIcon>
                <ListItemText>Share on LinkedIn</ListItemText>
              </MenuItem>
              <MenuItem onClick={shareToFacebook}>
                <ListItemIcon>
                  <FacebookIcon size={16} />
                </ListItemIcon>
                <ListItemText>Share on Facebook</ListItemText>
              </MenuItem>
              <MenuItem onClick={() => void copyLink()}>
                <ListItemIcon>
                  <Link2 size={16} />
                </ListItemIcon>
                <ListItemText>Copy link</ListItemText>
              </MenuItem>
            </Menu>
          </Box>
        </Box>
      </Box>

      {bannerImage && (
        <Box sx={{mt: 5, width: '100%', height: {xs: 200, md: 300}, position: 'relative', overflow: 'hidden'}}>
          <Box
            component="img"
            src={bannerImage}
            alt={metadata.title}
            sx={{width: '100%', height: '100%', objectFit: 'cover'}}
          />
        </Box>
      )}
    </Box>
  );
}
