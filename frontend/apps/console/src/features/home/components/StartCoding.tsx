/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Box, Stack, Tooltip, Typography, IconButton, useColorScheme} from '@wso2/oxygen-ui';
import {ChevronLeft, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useRef, useState, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TechnologyBasedApplicationTemplateMetadata from '../../applications/config/TechnologyBasedApplicationTemplateMetadata';

export default function StartCoding(): JSX.Element {
  const navigate = useNavigate();
  const {mode} = useColorScheme();
  const {t} = useTranslation();
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const [showLeftArrow, setShowLeftArrow] = useState(false);
  const [showRightArrow, setShowRightArrow] = useState(false);

  const handleFrameworkClick = (frameworkId: string) => {
    // Navigate to applications page with the selected type as a query parameter
    navigate(`/applications/types?type=${frameworkId}`)?.catch(() => undefined);
  };

  const checkScroll = () => {
    if (scrollContainerRef.current) {
      const {scrollLeft, scrollWidth, clientWidth} = scrollContainerRef.current;
      setShowLeftArrow(scrollLeft > 0);
      setShowRightArrow(scrollLeft < scrollWidth - clientWidth - 1);
    }
  };

  const scroll = (direction: 'left' | 'right') => {
    if (scrollContainerRef.current) {
      const scrollAmount = 200;
      scrollContainerRef.current.scrollBy({
        left: direction === 'left' ? -scrollAmount : scrollAmount,
        behavior: 'smooth',
      });
    }
  };

  useEffect(() => {
    checkScroll();
    const container = scrollContainerRef.current;
    container?.addEventListener('scroll', checkScroll);
    window.addEventListener('resize', checkScroll);
    return () => {
      container?.removeEventListener('scroll', checkScroll);
      window.removeEventListener('resize', checkScroll);
    };
  }, []);

  return (
    <Box>
      <Stack spacing={2}>
        <Box sx={{position: 'relative', display: 'flex', alignItems: 'center', gap: 1}}>
          {/* Left Gradient Fade */}
          {showLeftArrow && (
            <Box
              sx={{
                position: 'absolute',
                left: 0,
                top: 0,
                bottom: 0,
                width: 60,
                zIndex: 1,
                pointerEvents: 'none',
                background: (theme) =>
                  `linear-gradient(to right, ${mode === 'dark' ? theme.palette.common.black : theme.palette.common.white} 0%, transparent 100%)`,
              }}
            />
          )}

          {/* Left Arrow */}
          {showLeftArrow && (
            <IconButton
              size="small"
              onClick={() => scroll('left')}
              sx={{
                position: 'absolute',
                left: -8,
                zIndex: 2,
                bgcolor: 'background.paper',
                border: '1px solid',
                borderColor: 'divider',
                boxShadow: 1,
                '&:hover': {
                  bgcolor: 'action.hover',
                },
              }}
            >
              <ChevronLeft />
            </IconButton>
          )}

          {/* Scrollable Container */}
          <Box
            ref={scrollContainerRef}
            sx={{
              display: 'flex',
              gap: 1.5,
              alignItems: 'center',
              overflowX: 'auto',
              scrollbarWidth: 'none',
              '&::-webkit-scrollbar': {
                display: 'none',
              },
              flexGrow: 1,
            }}
          >
            {/* Framework Icons */}
            {TechnologyBasedApplicationTemplateMetadata.map((template) => {
              const title = t(template.titleKey);
              const isEnabled = !template.disabled;

              return (
                <Tooltip key={template.value} title={title} arrow placement="top">
                  <Box
                    component="button"
                    onClick={() => handleFrameworkClick(template.value)}
                    disabled={!isEnabled}
                    sx={{
                      display: 'flex',
                      flexDirection: 'row',
                      alignItems: 'center',
                      gap: 1,
                      px: 2,
                      py: 1,
                      border: '1px solid',
                      borderColor: 'divider',
                      borderRadius: 1,
                      bgcolor: 'background.paper',
                      cursor: isEnabled ? 'pointer' : 'not-allowed',
                      transition: 'all 0.2s',
                      opacity: isEnabled ? 1 : 0.3,
                      flexShrink: 0,
                      '&:hover': isEnabled
                        ? {
                            borderColor: 'primary.main',
                            bgcolor: 'action.hover',
                          }
                        : {},
                      '&:disabled': {
                        cursor: 'not-allowed',
                      },
                    }}
                  >
                    <Box sx={{width: 20, height: 20, display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
                      {template.icon}
                    </Box>
                    <Typography
                      variant="caption"
                      sx={{
                        fontSize: '0.875rem',
                        color: isEnabled ? 'text.primary' : 'text.disabled',
                        fontWeight: 400,
                      }}
                    >
                      {title}
                    </Typography>
                  </Box>
                </Tooltip>
              );
            })}
          </Box>

          {/* Right Gradient Fade */}
          {showRightArrow && (
            <Box
              sx={{
                position: 'absolute',
                right: 0,
                top: 0,
                bottom: 0,
                width: 60,
                zIndex: 1,
                pointerEvents: 'none',
                background: (theme) =>
                  `linear-gradient(to left, ${mode === 'dark' ? theme.palette.common.black : theme.palette.common.white} 0%, transparent 100%)`,
              }}
            />
          )}

          {/* Right Arrow */}
          {showRightArrow && (
            <IconButton
              size="small"
              onClick={() => scroll('right')}
              sx={{
                position: 'absolute',
                right: -8,
                zIndex: 2,
                bgcolor: 'background.paper',
                border: '1px solid',
                borderColor: 'divider',
                boxShadow: 1,
                '&:hover': {
                  bgcolor: 'action.hover',
                },
              }}
            >
              <ChevronRight />
            </IconButton>
          )}
        </Box>
      </Stack>
    </Box>
  );
}
