// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Editor, {type OnMount} from '@monaco-editor/react';
import {JsonLogo, JwtLogo} from '@thunderid/components';
import {getCspNonce} from '@thunderid/design';
import {Stack, Typography, Box} from '@wso2/oxygen-ui';
import type {editor as MonacoEditor, IDisposable, IRange} from 'monaco-editor';
import {useRef, useEffect, useState, useCallback} from 'react';

/**
 * Descriptions for standard JWT claims, shown as hover tooltips in the preview.
 */
const JWT_CLAIM_DESCRIPTIONS: Record<string, string> = {
  aud: 'Audience — who or what the token is intended for',
  client_id: 'Client ID that requested this token',
  exp: 'Expiration time — Unix timestamp when the token expires',
  grant_type: 'OAuth 2 grant type used to obtain this token',
  iat: 'Issued at — Unix timestamp when the token was created',
  iss: 'Issuer — entity that created and signed the token',
  jti: 'JWT ID — unique identifier for this specific token',
  nbf: 'Not before — token not valid before this Unix timestamp',
  scope: 'OAuth 2 scopes granted to the token',
  sub: 'Subject — the principal (user) this token represents',
};

interface MonacoLike {
  Range: new (startLineNumber: number, startColumn: number, endLineNumber: number, endColumn: number) => IRange;
}

/**
 * Props for the {@link JwtPreview} component.
 */
interface JwtPreviewProps {
  /**
   * The payload object to render as formatted JSON. Values are normally placeholder
   * strings, but a claim like `act` may be a nested object.
   */
  payload: Record<string, unknown>;
  /**
   * Claims that are always present by default. These are highlighted with a
   * dotted underline and show a descriptive tooltip on hover (like jwt.io).
   * Ignored when format is 'json'.
   */
  defaultClaims?: readonly string[];
  /**
   * Optional JOSE header to display above the payload.
   * When provided, the preview shows a "HEADER" and "PAYLOAD" section.
   * Ignored when format is 'json'.
   */
  header?: Record<string, string>;
  /**
   * Display format. 'jwt' (default) shows the JWT logo with optional header/payload
   * sections and claim decorations. 'json' shows the JSON logo and renders the
   * payload as a plain JSON object without JWT-specific annotations.
   */
  format?: 'jwt' | 'json';
}

/**
 * Renders a JWT payload preview with the JWT logo, a label, and a Monaco JSON viewer.
 *
 * Default claims are highlighted in purple with a dotted underline. Hovering over
 * a default claim key shows a tooltip describing its purpose — the same experience
 * as jwt.io.
 *
 * @param props - Component props
 * @returns A bordered box containing the JWT logo, title, and annotated JSON preview
 */
export default function JwtPreview({payload, defaultClaims = [], header = undefined, format = 'jwt'}: JwtPreviewProps) {
  const editorRef = useRef<MonacoEditor.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<MonacoLike | null>(null);
  const decorationIdsRef = useRef<string[]>([]);
  const defaultClaimsRef = useRef<readonly string[]>(defaultClaims);
  const contentListenerRef = useRef<IDisposable | null>(null);
  const sizeListenerRef = useRef<IDisposable | null>(null);
  const [measuredPayloadHeight, setMeasuredPayloadHeight] = useState<number | null>(null);

  const applyDecorations = useCallback(() => {
    if (format === 'json') return;
    const editorInstance = editorRef.current;
    const monacoInstance = monacoRef.current;
    if (!editorInstance || !monacoInstance) return;

    const model = editorInstance.getModel();
    if (!model) return;

    const claims = defaultClaimsRef.current;
    if (claims.length === 0) {
      decorationIdsRef.current = editorInstance.deltaDecorations(decorationIdsRef.current, []);
      return;
    }

    const lines = model.getValue().split('\n');
    const newDecorations: MonacoEditor.IModelDeltaDecoration[] = [];

    lines.forEach((line, i) => {
      claims.forEach((claim) => {
        const idx = line.indexOf(`"${claim}"`);
        if (idx === -1) return;
        // Highlight the claim name text inside the quotes
        newDecorations.push({
          range: new monacoInstance.Range(i + 1, idx + 2, i + 1, idx + claim.length + 2),
          options: {
            inlineClassName: 'jwt-default-claim',
            hoverMessage: {
              value: `**${claim}**\n\n${JWT_CLAIM_DESCRIPTIONS[claim] ?? 'Standard JWT claim'}`,
            },
          },
        });
      });
    });

    decorationIdsRef.current = editorInstance.deltaDecorations(decorationIdsRef.current, newDecorations);
  }, [format]);

  // Keep defaultClaimsRef in sync so the content-change listener always sees latest claims
  useEffect(() => {
    defaultClaimsRef.current = defaultClaims;
    // Re-apply if editor is already mounted (handles claims prop changes)
    if (editorRef.current && monacoRef.current) {
      applyDecorations();
    }
  }, [defaultClaims, applyDecorations]);

  const handleMount: OnMount = (editorInstance, monacoInstance) => {
    editorRef.current = editorInstance;
    monacoRef.current = monacoInstance as MonacoLike;

    // Apply decorations on initial mount
    applyDecorations();

    // Re-apply decorations whenever the content is updated (e.g. when payload prop changes)
    contentListenerRef.current?.dispose();
    contentListenerRef.current = editorInstance.onDidChangeModelContent(() => {
      applyDecorations();
    });

    // Follow the height the editor actually needs, so wrapped values are never cut off.
    setMeasuredPayloadHeight(editorInstance.getContentHeight());
    sizeListenerRef.current?.dispose();
    sizeListenerRef.current = editorInstance.onDidContentSizeChange((event) => {
      setMeasuredPayloadHeight(event.contentHeight);
    });
  };

  // Clean up editor listeners on unmount
  useEffect(
    () => () => {
      contentListenerRef.current?.dispose();
      sizeListenerRef.current?.dispose();
    },
    [],
  );

  const isJson = format === 'json';
  const headerJson = !isJson && header ? JSON.stringify(header, null, 2) : null;
  const payloadJson = JSON.stringify(payload, null, 2);
  const headerHeight = headerJson ? Math.min(200, Math.max(60, headerJson.split('\n').length * 20 + 16)) : 0;
  // Line count alone under-measures: the rendered line height differs from any constant we pick,
  // and wrapped values occupy more than one line each. The editor reports what it actually needs,
  // so the estimate is only the starting height until it does.
  const payloadHeight = Math.min(600, Math.max(80, measuredPayloadHeight ?? payloadJson.split('\n').length * 20 + 16));

  const editorOptions = {
    readOnly: true,
    minimap: {enabled: false},
    scrollBeyondLastLine: false,
    automaticLayout: true,
    fontSize: 14,
    tabSize: 2,
    lineNumbers: 'off' as const,
    folding: false,
    contextmenu: false,
    renderLineHighlight: 'none' as const,
    wordWrap: 'on' as const,
    scrollbar: {
      vertical: 'auto' as const,
      // Lines wrap, so there is nothing to scroll horizontally.
      horizontal: 'hidden' as const,
      verticalScrollbarSize: 8,
      horizontalScrollbarSize: 0,
      handleMouseWheel: true,
      // Once the preview reaches its end the wheel goes back to the page, so scrolling past the
      // preview never traps the cursor.
      alwaysConsumeMouseWheel: false,
    },
    guides: {
      indentation: false,
    },
  };

  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        p: 2,
        height: '100%',
        // The editor measures its own container, so without these it can widen the column it sits
        // in rather than scrolling inside it.
        minWidth: 0,
        overflow: 'hidden',
      }}
    >
      {/* Styling for default claim annotations */}
      {!isJson && (
        <style nonce={getCspNonce()}>
          {`.jwt-default-claim { color: #C586C0 !important; text-decoration: underline dotted; text-decoration-color: #C586C0; cursor: help; }`}
        </style>
      )}
      <Stack spacing={2}>
        <Stack direction="row" spacing={1} alignItems="center">
          {isJson ? <JsonLogo /> : <JwtLogo />}
        </Stack>

        {headerJson && (
          <>
            <Typography variant="caption" color="text.secondary" sx={{textTransform: 'uppercase', letterSpacing: 1}}>
              Decoded Header
            </Typography>
            <Box sx={{overflow: 'hidden', borderRadius: 1}}>
              <Editor
                height={headerHeight}
                language="json"
                theme="vs-dark"
                value={headerJson}
                options={editorOptions}
              />
            </Box>
          </>
        )}

        {headerJson && (
          <Typography variant="caption" color="text.secondary" sx={{textTransform: 'uppercase', letterSpacing: 1}}>
            Decoded Payload
          </Typography>
        )}
        <Box sx={{overflow: 'hidden', borderRadius: 1}}>
          <Editor
            height={payloadHeight}
            language="json"
            theme="vs-dark"
            value={payloadJson}
            onMount={handleMount}
            options={editorOptions}
          />
        </Box>
      </Stack>
    </Box>
  );
}
