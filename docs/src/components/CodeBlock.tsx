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

import React, {type JSX, ReactNode} from 'react';
import DocusaurusCodeBlock from '@theme/CodeBlock';

interface CodeBlockProps {
  /**
   * The programming language for syntax highlighting
   */
  lang?: string;
  /**
   * Label for the code block tab (used by CodeGroup component)
   */
  // eslint-disable-next-line react/no-unused-prop-types
  label?: string;
  /**
   * The code content
   */
  children: string | ReactNode;
  /**
   * Title for the code block
   */
  title?: string;
  /**
   * Whether to show line numbers
   */
  showLineNumbers?: boolean;
}

/**
 * CodeBlock component for displaying code with syntax highlighting
 *
 * @example
 * ```tsx
 * <CodeBlock lang="bash" label="npm">
 *   npm install @example/react
 * </CodeBlock>
 * ```
 */
export default function CodeBlock({
  lang = 'text',
  children,
  title = undefined,
  showLineNumbers = undefined,
}: CodeBlockProps): JSX.Element {
  // Extract text content from children (handles React elements from MDX)
  const getTextContent = (node: ReactNode): string => {
    if (typeof node === 'string') {
      return node;
    }

    if (typeof node === 'number') {
      return String(node);
    }

    if (Array.isArray(node)) {
      return node.map(getTextContent).join('');
    }

    if (React.isValidElement(node)) {
      const props = node.props as {children?: ReactNode};
      if (props.children) {
        return getTextContent(props.children);
      }
    }

    return '';
  };

  const code = getTextContent(children).trim();

  return (
    <DocusaurusCodeBlock language={lang} title={title} showLineNumbers={showLineNumbers}>
      {code}
    </DocusaurusCodeBlock>
  );
}
