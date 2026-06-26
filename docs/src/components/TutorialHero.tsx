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

import {Grid, Typography, List, ListItem, ListItemIcon, ListItemText, Box} from '@wso2/oxygen-ui';
import {Box as Cube} from '@wso2/oxygen-ui-icons-react';
import {ReactNode, Children, isValidElement} from 'react';

interface TutorialHeroProps {
  children: ReactNode;
}

interface TutorialHeroItemProps {
  icon?: ReactNode;
  children: ReactNode;
}

interface SectionData {
  title: string;
  content: ReactNode[];
}

const ICON_SIZE = 22;
const ICON_CONTAINER_SIZE = 38;

const iconContainerSx = {
  alignItems: 'center',
  borderRadius: 'var(--oxygen-shape-borderRadius)',
  display: 'flex',
  height: ICON_CONTAINER_SIZE,
  justifyContent: 'center',
  width: ICON_CONTAINER_SIZE,
} as const;

const iconInnerSx = {
  alignItems: 'center',
  color: 'primary.main',
  display: 'flex',
  height: ICON_SIZE,
  justifyContent: 'center',
  width: ICON_SIZE,
} as const;

// TutorialHeroItem component - used in MDX to pass custom icons
export function TutorialHeroItem({icon = undefined, children}: TutorialHeroItemProps) {
  return (
    <ListItem sx={{}}>
      <ListItemIcon sx={{minWidth: ICON_CONTAINER_SIZE + 8}}>
        <Box sx={iconContainerSx}>
          <Box sx={iconInnerSx}>
            {icon ?? <Cube />}
          </Box>
        </Box>
      </ListItemIcon>
      <ListItemText primary={children} />
    </ListItem>
  );
}

// Convert content to list items with icons
function renderContentWithIcons(content: ReactNode[]): ReactNode {
  return content.map((item, index) => {
    if (isValidElement(item)) {
      const elementKey = item.key ?? `content-${index}`;

      // Check if it's a TutorialHeroItem
      const isTutorialHeroItem =
        item.type === TutorialHeroItem ||
        (typeof item.type === 'function' && item.type.name === 'TutorialHeroItem') ||
        item.props?.mdxType === 'TutorialHeroItem';

      if (isTutorialHeroItem) {
        return (
          <List key={elementKey} sx={{py: 0}}>
            {item}
          </List>
        );
      }

      // Check if it's a ul/ol list (fallback for markdown lists)
      const isList =
        item.type === 'ul' ||
        item.type === 'ol' ||
        (typeof item.type === 'function' && (item.type.name === 'ul' || item.type.name === 'ol')) ||
        item.props?.mdxType === 'ul' ||
        item.props?.mdxType === 'ol';

      if (isList && item.props.children) {
        const listItems = Children.toArray(item.props.children as ReactNode).filter((child) => isValidElement(child));

        return (
          <List key={elementKey} sx={{py: 0}}>
            {listItems.map((listItem) => {
              if (isValidElement(listItem)) {
                const text = extractTextFromChildren(listItem.props.children as ReactNode);
                return (
                  <ListItem key={listItem.key ?? text} sx={{px: 0, py: 0.5}}>
                    <ListItemIcon sx={{minWidth: 40}}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                          width: 32,
                          height: 32,
                          borderRadius: 1,
                          bgcolor: 'primary.lighter',
                          color: 'primary.main',
                        }}
                      >
                        <Cube />
                      </Box>
                    </ListItemIcon>
                    <ListItemText primary={text} />
                  </ListItem>
                );
              }
              return null;
            })}
          </List>
        );
      }
    }
    return <Box key={extractTextFromChildren(item)}>{item}</Box>;
  });
}

export default function TutorialHero({children}: TutorialHeroProps) {
  const sections: SectionData[] = [];
  let currentSection: SectionData | null = null;

  // Process children to group them into sections
  Children.forEach(children, (child) => {
    if (!isValidElement(child)) {
      if (currentSection) {
        currentSection.content.push(child);
      }
      return;
    }

    // Check if this is an h2 heading that should become a section
    const isHeading =
      child.type === 'h2' ||
      (typeof child.type === 'function' && child.type.name === 'h2') ||
      child.props?.mdxType === 'h2';

    if (isHeading) {
      // Save previous section if it exists
      if (currentSection) {
        sections.push(currentSection);
      }
      // Create new section
      currentSection = {
        title:
          typeof child.props.children === 'string'
            ? (child.props.children as string)
            : extractTextFromChildren(child.props.children as ReactNode),
        content: [],
      };
    } else if (currentSection) {
      // Add content to current section
      currentSection.content.push(child);
    }
  });

  // Push the last section
  if (currentSection) {
    sections.push(currentSection);
  }

  return (
    <Grid container spacing={3} sx={{mb: 4}}>
      {sections.map((section) => (
        <Grid size={{xs: 12, md: 6}} key={section.title}>
          <Box sx={{height: '100%'}}>
            <Typography
              variant="h6"
              gutterBottom
              sx={{
                mb: 2,
                pl: 2,
                borderLeft: '4px solid',
                borderColor: 'primary.main',
              }}
            >
              {section.title}
            </Typography>
            {renderContentWithIcons(section.content)}
          </Box>
        </Grid>
      ))}
    </Grid>
  );
}

// Helper function to extract text from React children
function extractTextFromChildren(children: ReactNode): string {
  if (typeof children === 'string') {
    return children;
  }
  if (Array.isArray(children)) {
    return children.map(extractTextFromChildren).join('');
  }
  if (isValidElement(children) && children.props.children) {
    return extractTextFromChildren(children.props.children as ReactNode);
  }
  return '';
}
