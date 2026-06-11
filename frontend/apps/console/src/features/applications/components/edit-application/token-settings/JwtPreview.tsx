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

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {editor as MonacoEditor, IDisposable, IRange} from 'monaco-editor';
import {useRef, useEffect} from 'react';
import Editor, {type OnMount} from '@/lib/MonacoEditor';

/**
 * Descriptions for standard JWT claims, shown as hover tooltips in the preview.
 */
const JWT_CLAIM_DESCRIPTIONS: Record<string, string> = {
  aud: 'Audience — who or what the token is intended for',
  client_id: 'Client ID that requested this token',
  exp: 'Expiration time — Unix timestamp when the token expires',
  grant_type: 'OAuth 2.0 grant type used to obtain this token',
  iat: 'Issued at — Unix timestamp when the token was created',
  iss: 'Issuer — entity that created and signed the token',
  jti: 'JWT ID — unique identifier for this specific token',
  nbf: 'Not before — token not valid before this Unix timestamp',
  scope: 'OAuth 2.0 scopes granted to the token',
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
   * The label displayed next to the JWT logo.
   */
  title: string;
  /**
   * The JWT payload object to render as formatted JSON.
   */
  payload: Record<string, string>;
  /**
   * Claims that are always present by default. These are highlighted with a
   * dotted underline and show a descriptive tooltip on hover (like jwt.io).
   */
  defaultClaims?: readonly string[];
}

/**
 * JWT logo SVG — combines the geometric icon and the "JWT" wordmark.
 */
function JwtLogo() {
  return (
    <Box
      component="svg"
      width="90"
      height="32"
      viewBox="0 0 90 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      sx={{flexShrink: 0, height: 20}}
    >
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M18.3683 8.60806V0H13.5682V8.60806L15.9683 11.9041L18.3683 8.60806Z"
        fill="#191919"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M13.5682 23.3919V32H18.3683V23.3919L15.9683 20.0959L13.5682 23.3919Z"
        fill="#191919"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M18.3684 23.3928L23.4244 30.3689L27.3285 27.5208L22.2404 20.5768L18.3684 19.2968V23.3928Z"
        fill="#3EC6EB"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M13.5682 8.60823L8.51218 1.63218L4.60815 4.4802L9.69619 11.4243L13.5682 12.7043V8.60823Z"
        fill="#3EC6EB"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M9.69607 11.4244L1.50401 8.76841L0 13.3444L8.19206 16.0005L12.0961 14.7525L9.69607 11.4244Z"
        fill="#FF44DD"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M19.8402 17.2478L22.2402 20.5758L30.4323 23.2319L31.9363 18.6558L23.7442 15.9998L19.8402 17.2478Z"
        fill="#FF44DD"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M23.7442 16.0005L31.9363 13.3444L30.4323 8.76841L22.2402 11.4244L19.8402 14.7525L23.7442 16.0005Z"
        fill="#635DFF"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M8.19206 15.9998L0 18.6558L1.50401 23.2319L9.69607 20.5758L12.0961 17.2478L8.19206 15.9998Z"
        fill="#635DFF"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M9.69619 20.5768L4.60815 27.5208L8.51218 30.3689L13.5682 23.3928V19.2968L9.69619 20.5768Z"
        fill="#FF4F40"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M22.2404 11.4243L27.3285 4.4802L23.4244 1.63218L18.3684 8.60823V12.7043L22.2404 11.4243Z"
        fill="#FF4F40"
      />
      <path d="M46 6V18C46 21.3137 43.3137 24 40 24V24" stroke="currentColor" strokeWidth="4" />
      <path
        d="M52.8932 6V20.5C52.8932 22.433 54.4602 24 56.3932 24V24C58.3262 24 59.8932 22.433 59.8932 20.5V11C59.8932 9.34315 61.2363 8 62.8932 8V8C64.55 8 65.8932 9.34315 65.8932 11V20.5C65.8932 22.433 67.4602 24 69.3932 24V24C71.3262 24 72.8932 22.433 72.8932 20.5V6"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path d="M77.8932 8H83.8932M83.8932 8V26M83.8932 8H89.8932" stroke="currentColor" strokeWidth="4" />
    </Box>
  );
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
export default function JwtPreview({title, payload, defaultClaims = []}: JwtPreviewProps) {
  const editorRef = useRef<MonacoEditor.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<MonacoLike | null>(null);
  const decorationIdsRef = useRef<string[]>([]);
  const defaultClaimsRef = useRef<readonly string[]>(defaultClaims);
  const contentListenerRef = useRef<IDisposable | null>(null);

  const applyDecorations = () => {
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
  };

  // Keep defaultClaimsRef in sync so the content-change listener always sees latest claims
  useEffect(() => {
    defaultClaimsRef.current = defaultClaims;
    // Re-apply if editor is already mounted (handles claims prop changes)
    if (editorRef.current && monacoRef.current) {
      applyDecorations();
    }
  }, [defaultClaims]);

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
  };

  // Clean up content listener on unmount
  useEffect(
    () => () => {
      contentListenerRef.current?.dispose();
    },
    [],
  );

  const jsonValue = JSON.stringify(payload, null, 2);
  // Size the editor to fit content (capped at 600px to match the previous maxHeight)
  const editorHeight = Math.min(600, Math.max(80, jsonValue.split('\n').length * 20 + 16));

  return (
    <Box
      sx={{
        bgcolor: 'background.paper',
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        p: 2,
        height: '100%',
      }}
    >
      {/* Styling for default claim annotations */}
      <style>{`.jwt-default-claim { color: #C586C0 !important; text-decoration: underline dotted; text-decoration-color: #C586C0; cursor: help; }`}</style>
      <Stack spacing={2}>
        <Stack direction="row" spacing={1} alignItems="center">
          <JwtLogo />
          <Typography variant="body1">{title}</Typography>
        </Stack>
        <Box sx={{overflow: 'hidden', borderRadius: 1}}>
          <Editor
            height={editorHeight}
            language="json"
            theme="vs-dark"
            value={jsonValue}
            onMount={handleMount}
            options={{
              readOnly: true,
              minimap: {enabled: false},
              scrollBeyondLastLine: false,
              automaticLayout: true,
              fontSize: 14,
              tabSize: 2,
              lineNumbers: 'off',
              folding: false,
              contextmenu: false,
              renderLineHighlight: 'none',
              wordWrap: 'on',
              scrollbar: {
                vertical: 'hidden',
                horizontal: 'hidden',
                verticalScrollbarSize: 0,
                horizontalScrollbarSize: 0,
                handleMouseWheel: false, // 🚫 disables wheel scroll
                alwaysConsumeMouseWheel: false,
              },
              guides: {
                indentation: false,
              },
            }}
          />
        </Box>
      </Stack>
    </Box>
  );
}
