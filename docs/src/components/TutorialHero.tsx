// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Grid, Typography, List, ListItem, ListItemIcon, ListItemText, Box, styled} from '@wso2/oxygen-ui';
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

const ICON_SIZE = 18;
const ICON_CONTAINER_SIZE = 34;

const SectionCard = styled('div')({
  background: 'var(--oxygen-palette-background-paper)',
  border: '1px solid var(--ifm-color-emphasis-200)',
  borderRadius: '0.875rem',
  height: '100%',
  padding: '1.5rem',
  '[data-theme="dark"] &': {
    borderColor: 'rgba(255, 255, 255, 0.09)',
  },
});

const iconContainerSx = {
  alignItems: 'center',
  bgcolor: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.10)',
  borderRadius: '0.5rem',
  display: 'flex',
  flexShrink: 0,
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
    <ListItem sx={{px: 0, py: 0.75}}>
      <ListItemIcon sx={{minWidth: 'auto', mr: 1.75}}>
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
                  <ListItem key={listItem.key ?? text} sx={{px: 0, py: 0.75}}>
                    <ListItemIcon sx={{minWidth: 'auto', mr: 1.75}}>
                      <Box sx={iconContainerSx}>
                        <Box sx={iconInnerSx}>
                          <Cube />
                        </Box>
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
          <SectionCard>
            <Typography
              sx={{
                mb: 2,
                fontFamily: 'var(--ifm-font-family-monospace)',
                fontSize: '0.6875rem',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.1em',
                color: 'text.secondary',
              }}
            >
              {section.title}
            </Typography>
            {renderContentWithIcons(section.content)}
          </SectionCard>
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
