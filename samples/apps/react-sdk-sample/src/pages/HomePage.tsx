/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { SignOutButton, useThunderID } from "@thunderid/react";
import {
  Avatar,
  Box,
  Divider,
  IconButton,
  Layout,
  Menu,
  MenuItem,
  Typography,
} from "@wso2/oxygen-ui";
import { useEffect, useState } from "react";
import { jwtDecode } from "jwt-decode";

interface DecodedToken {
  header?: {
    alg?: string;
    typ?: string;
    kid?: string;
  };
  payload?: Record<string, unknown>;
  signature?: string;
}

// Helper component to render formatted JSON with syntax highlighting
const JsonDisplay = ({ data }: { data: unknown }) => {
  const formatJson = (obj: unknown): React.ReactNode[] => {
    const json = JSON.stringify(obj, null, 2);
    const lines = json.split("\n");

    return lines.map((line, index) => {
      // Preserve leading spaces for indentation
      const leadingSpaces = line.match(/^(\s*)/)?.[1] || "";
      const trimmedLine = line.trimStart();

      let formattedLine = trimmedLine;
      let color = "#e0e0e0"; // default color for white text

      // Color keys
      if (trimmedLine.includes(":")) {
        const keyMatch = trimmedLine.match(/"([^"]+)":/);
        if (keyMatch) {
          const key = keyMatch[0];
          formattedLine = trimmedLine.replace(
            key,
            `<span style="color: #79c0ff;">${key}</span>`
          );
          color = "#e0e0e0";
        }
      }

      // Color string values
      formattedLine = formattedLine.replace(
        /: "([^"]*)"/g,
        ': <span style="color: #a5d6ff;">"$1"</span>'
      );

      // Color numbers
      formattedLine = formattedLine.replace(
        /: (\d+)/g,
        ': <span style="color: #79c0ff;">$1</span>'
      );

      // Color booleans
      formattedLine = formattedLine.replace(
        /: (true|false)/g,
        ': <span style="color: #ff7b72;">$1</span>'
      );

      // Color null
      formattedLine = formattedLine.replace(
        /: (null)/g,
        ': <span style="color: #ff7b72;">$1</span>'
      );

      // Reconstruct line with preserved indentation
      const indent = leadingSpaces.replace(/ /g, "&nbsp;");
      const finalLine = indent + formattedLine;

      return (
        <div
          key={index}
          dangerouslySetInnerHTML={{ __html: finalLine }}
          style={{ color }}
        />
      );
    });
  };

  return (
    <div style={{ lineHeight: "1.6", whiteSpace: "pre" }}>
      {formatJson(data)}
    </div>
  );
};

const HomePage = () => {
  const { getAccessToken, signIn } = useThunderID();
  const [token, setToken] = useState<string | null>(null);
  const [decodedToken, setDecodedToken] = useState<DecodedToken | null>(null);
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleMenuOpen = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  useEffect(() => {
    const fetchTokens = async () => {
      try {
        const accessToken = await getAccessToken();
        setToken(accessToken);

        // Decode the access token
        if (accessToken) {
          const decoded = jwtDecode<Record<string, unknown>>(accessToken);

          // Split the token to get header and signature
          const [headerB64, , signature] = accessToken.split(".");
          const header = JSON.parse(atob(headerB64));

          setDecodedToken({
            header,
            payload: decoded,
            signature,
          });
        }
      } catch (error) {
        console.error("Error fetching tokens:", error);
      }
    };

    fetchTokens();
  }, [getAccessToken]);

  // Extract username from decoded token
  const username =
    (decodedToken?.payload?.username as string) ||
    (decodedToken?.payload?.email as string) ||
    (decodedToken?.payload?.sub as string) ||
    "User";

  // Extract name information from token
  const givenName = decodedToken?.payload?.given_name as string | undefined;
  const familyName = decodedToken?.payload?.family_name as string | undefined;

  // Construct display name
  const displayName =
    givenName && familyName
      ? `${givenName} ${familyName}`
      : givenName || familyName || username;

  // Get initials for avatar
  const getInitials = (name: string) => {
    // If we have both given and family name, use their first letters
    if (givenName && familyName) {
      return (givenName[0] + familyName[0]).toUpperCase();
    }

    const parts = name.split(/[@._\s]/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return name.substring(0, 2).toUpperCase();
  };

  return (
    <Layout>
      <Box className="home-container">
        {token ? (
          <Box className="token-container">
            {/* Header with Avatar */}
            <Box
              sx={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                mb: 4,
                pb: 2,
                borderBottom: "1px solid rgba(255, 255, 255, 0.12)",
              }}
            >
              <Typography variant="h6">Hello {displayName}!</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                <Typography variant="body1">{username}</Typography>
                <IconButton
                  onClick={handleMenuOpen}
                  sx={{ p: 0 }}
                  aria-controls={open ? "account-menu" : undefined}
                  aria-haspopup="true"
                  aria-expanded={open ? "true" : undefined}
                >
                  <Avatar sx={{ bgcolor: "#ff7043", cursor: "pointer" }}>
                    {getInitials(displayName)}
                  </Avatar>
                </IconButton>
                <Menu
                  id="account-menu"
                  anchorEl={anchorEl}
                  open={open}
                  onClose={handleMenuClose}
                  onClick={handleMenuClose}
                  transformOrigin={{ horizontal: "right", vertical: "top" }}
                  anchorOrigin={{ horizontal: "right", vertical: "bottom" }}
                >
                  <SignOutButton>
                    {({ signOut, isLoading }) => (
                      <MenuItem
                        onClick={async () => {
                          await signOut();
                          await signIn();
                        }}
                        disabled={isLoading}
                      >
                        Sign Out
                      </MenuItem>
                    )}
                  </SignOutButton>
                </Menu>
              </Box>
            </Box>

            <Typography variant="h5" sx={{ mb: 3 }}>
              Access Token
            </Typography>
            <Box
              sx={{
                maxWidth: "100%",
                overflowX: "auto",
                backgroundColor: "#1e1e1e",
                padding: 2,
                borderRadius: 1,
              }}
            >
              <pre
                style={{
                  margin: 0,
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-all",
                  fontFamily: "monospace",
                  fontSize: "0.875rem",
                  color: "#a5d6ff",
                }}
              >
                <code>{token}</code>
              </pre>
            </Box>
            <Divider sx={{ my: 2 }} />
            {decodedToken && (
              <Box>
                <Typography variant="h5" sx={{ mb: 3 }}>
                  Decoded Token
                </Typography>
                <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
                  {/* Header and Payload Side-by-Side */}
                  <Box
                    sx={{
                      display: "flex",
                      gap: 3,
                      flexWrap: "wrap",
                      alignItems: "stretch",
                    }}
                  >
                    {/* Header Section */}
                    <Box
                      sx={{
                        flex: "1 1 300px",
                        minWidth: 0,
                        display: "flex",
                        flexDirection: "column",
                      }}
                    >
                      <Typography variant="h6" sx={{ mb: 1 }}>
                        Header:
                      </Typography>
                      <Box
                        sx={{
                          maxWidth: "100%",
                          overflowX: "auto",
                          backgroundColor: "#1e1e1e",
                          padding: 2,
                          borderRadius: 1,
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          textAlign: "left",
                          flexGrow: 1,
                        }}
                      >
                        <JsonDisplay data={decodedToken.header} />
                      </Box>
                    </Box>

                    {/* Payload Section */}
                    <Box
                      sx={{
                        flex: "1 1 300px",
                        minWidth: 0,
                        display: "flex",
                        flexDirection: "column",
                      }}
                    >
                      <Typography variant="h6" sx={{ mb: 1 }}>
                        Payload:
                      </Typography>
                      <Box
                        sx={{
                          maxWidth: "100%",
                          overflowX: "auto",
                          backgroundColor: "#1e1e1e",
                          padding: 2,
                          borderRadius: 1,
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          textAlign: "left",
                          flexGrow: 1,
                        }}
                      >
                        <JsonDisplay data={decodedToken.payload} />
                      </Box>
                    </Box>
                  </Box>

                  {/* Signature Section */}
                  <Box>
                    <Typography variant="h6" sx={{ mb: 1 }}>
                      Signature:
                    </Typography>
                    <Box
                      sx={{
                        maxWidth: "100%",
                        overflowX: "auto",
                        backgroundColor: "#1e1e1e",
                        padding: 2,
                        borderRadius: 1,
                        fontFamily: "monospace",
                        fontSize: "0.875rem",
                        textAlign: "left",
                      }}
                    >
                      <pre
                        style={{
                          margin: 0,
                          whiteSpace: "pre-wrap",
                          wordBreak: "break-all",
                          fontFamily: "monospace",
                          fontSize: "0.875rem",
                          color: "#a5d6ff",
                        }}
                      >
                        <code>{decodedToken.signature}</code>
                      </pre>
                    </Box>
                  </Box>
                </Box>
              </Box>
            )}
          </Box>
        ) : (
          <Typography>No token available. Please log in.</Typography>
        )}
      </Box>
    </Layout>
  );
};

export default HomePage;
