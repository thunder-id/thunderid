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

import {Box, IconButton, Typography} from '@wso2/oxygen-ui';
import {Check, Copy, Play} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';

interface TerminalBlockProps {
  command: string;
  tabs?: ReactNode;
}

export default function TerminalBlock({command, tabs = undefined}: TerminalBlockProps): JSX.Element {
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    void navigator.clipboard.writeText(command).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, overflow: 'hidden'}}>
      {tabs && <Box sx={{borderBottom: '1px solid', borderColor: 'divider'}}>{tabs}</Box>}
      <Box sx={{bgcolor: 'black', px: 2, py: 1.5, display: 'flex', alignItems: 'center', gap: 1}}>
        <Play size={12} style={{opacity: 0.4, color: '#fff', flexShrink: 0}} />
        <Typography variant="body2" fontFamily="monospace" sx={{flex: 1, color: 'success.light'}}>
          {command}
        </Typography>
        <IconButton
          size="small"
          aria-label="Copy command"
          onClick={handleCopy}
          sx={{color: copied ? 'success.light' : 'grey.400', '&:hover': {color: 'grey.100'}}}
        >
          {copied ? <Check size={14} /> : <Copy size={14} />}
        </IconButton>
      </Box>
    </Box>
  );
}
