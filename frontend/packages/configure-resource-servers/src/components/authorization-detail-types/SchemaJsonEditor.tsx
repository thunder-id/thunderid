// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Editor from '@monaco-editor/react';
import {Box, useColorScheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

export interface SchemaJsonEditorProps {
  /** The schema as JSON text. */
  value: string;
  /** Called with the new text on every edit. */
  onChange: (value: string) => void;
  /** Marks the editor as holding text that is not a schema the form can save. */
  invalid?: boolean;
  readOnly?: boolean;
  /** The accessible name of the editor. */
  ariaLabel?: string;
}

/**
 * Edits an authorization details type's JSON Schema as text. The form parses the text and reports
 * what is wrong with it, so this component only shows and edits it.
 */
export default function SchemaJsonEditor({
  value,
  onChange,
  invalid = false,
  readOnly = false,
  ariaLabel = undefined,
}: SchemaJsonEditorProps): JSX.Element {
  const {mode, systemMode} = useColorScheme();
  const colorMode = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  return (
    <Box
      sx={{
        height: 480,
        border: '1px solid',
        borderColor: invalid ? 'error.main' : 'divider',
        borderRadius: 1,
        overflow: 'hidden',
      }}
    >
      <Editor
        height="100%"
        language="json"
        theme={colorMode === 'dark' ? 'vs-dark' : 'vs'}
        value={value}
        onChange={(text) => onChange(text ?? '')}
        options={{
          readOnly,
          ariaLabel,
          minimap: {enabled: false},
          scrollBeyondLastLine: false,
          automaticLayout: true,
          fontSize: 12,
          tabSize: 2,
          wordWrap: 'on',
        }}
      />
    </Box>
  );
}
