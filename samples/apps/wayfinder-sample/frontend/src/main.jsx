// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ThunderIDProvider } from "@thunderid/react";
import { BrowserRouter } from "react-router";
import App from "./App.jsx";
import "./styles.css";
import { NativeAuthProvider } from "./auth/NativeAuthContext.jsx";
import { AUTH_CONFIG, SCOPES } from "./auth/config.js";
import { nativeVerboseAuthExtensions } from "./pages/NativeVerboseAuthPage.jsx";

const clientId = import.meta.env.VITE_THUNDERID_CLIENT_ID;
const appId = import.meta.env.VITE_THUNDERID_APP_ID || clientId;
const baseUrl = import.meta.env.VITE_THUNDERID_BASE_URL;
const thunderidReady = Boolean(clientId && baseUrl);
const afterSignInUrl = AUTH_CONFIG.isRedirectBased ? window.location.origin : "";

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <BrowserRouter>
      {thunderidReady ? (
        <ThunderIDProvider
          clientId={clientId}
          applicationId={appId}
          baseUrl={baseUrl}
          afterSignInUrl={afterSignInUrl}
          afterSignOutUrl={window.location.origin}
          scopes={SCOPES}
          extensions={nativeVerboseAuthExtensions}
          discovery={{ wellKnown: { enabled: true } }}
        >
          <NativeAuthProvider>
            <App authReady />
          </NativeAuthProvider>
        </ThunderIDProvider>
      ) : (
        <NativeAuthProvider>
          <App authReady={false} />
        </NativeAuthProvider>
      )}
    </BrowserRouter>
  </StrictMode>
);
