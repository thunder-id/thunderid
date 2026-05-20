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

import React, {ReactNode, Children, isValidElement} from 'react';
import {Box, Typography, styled} from '@wso2/oxygen-ui';

interface StepperProps {
  children: ReactNode;
  stepNode?: 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';
  as?: 'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6';
}

interface StepData {
  id?: string;
  label: string;
  content: ReactNode[];
}

const StepCircle = styled('div')<{ownerState: {active?: boolean}}>(({theme, ownerState}) => ({
  width: 36,
  height: 36,
  minWidth: 36,
  borderRadius: '50%',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: '0.95rem',
  flexShrink: 0,
  background: ownerState.active
    ? theme.palette.primary.main
    : theme.palette.mode === 'dark'
      ? theme.palette.grey[800]
      : theme.palette.grey[200],
  color: ownerState.active
    ? theme.palette.primary.contrastText
    : theme.palette.text.secondary,
  border: `2px solid ${ownerState.active ? theme.palette.primary.main : theme.palette.divider}`,
  boxShadow: ownerState.active ? `0 0 0 4px ${theme.palette.primary.main}22` : 'none',
}));

export default function Stepper({children, stepNode = 'h2', as = 'h2'}: StepperProps) {
  const steps: StepData[] = [];
  let currentStep: StepData | null = null;

  // Process children to group them into steps
  Children.forEach(children, (child: ReactNode) => {
    if (!isValidElement(child)) {
      if (currentStep) {
        currentStep.content.push(child);
      }
      return;
    }

    // Check if this is a heading that should become a step
    // In MDX, headings can be either string types (h1, h2, etc.) or components
    const isHeading =
      child.type === stepNode ||
      (typeof child.type === 'function' && child.type.name === stepNode) ||
      child.props?.mdxType === stepNode;

    if (isHeading) {
      // Save previous step if it exists
      if (currentStep) {
        steps.push(currentStep);
      }
      // Create new step, preserving the id Docusaurus generated for the heading
      currentStep = {
        id: child.props.id as string | undefined,
        label:
          typeof child.props.children === 'string'
            ? child.props.children
            : extractTextFromChildren(child.props.children),
        content: [],
      };
    } else if (currentStep) {
      // Add content to current step
      currentStep.content.push(child);
    }
  });

  // Push the last step
  if (currentStep) {
    steps.push(currentStep);
  }

  return (
    <Box sx={{mt: 4}}>
      {steps.map((step, index) => (
        <Box key={step.label} sx={{display: 'flex', gap: '16px'}}>
          {/* Left column: circle + connector line */}
          <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'center', flexShrink: 0}}>
            <StepCircle ownerState={{active: true}}>{index + 1}</StepCircle>
            {index < steps.length - 1 && (
              <Box sx={{width: '2px', flex: 1, mt: '6px', mb: '6px', background: 'rgba(128,128,128,0.4)', minHeight: '32px', borderRadius: '1px'}} />
            )}
          </Box>
          {/* Right column: title + content */}
          <Box sx={{flex: 1, minWidth: 0, pb: index < steps.length - 1 ? 4 : 0}}>
            <Typography
              id={step.id}
              variant={as}
              component="p"
              sx={{margin: 0, padding: 0, lineHeight: '36px', fontWeight: 700}}
            >
              {step.label}
            </Typography>
            <Box sx={{mt: 2}}>{step.content}</Box>
          </Box>
        </Box>
      ))}
    </Box>
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
    return extractTextFromChildren(children.props.children);
  }
  return '';
}
