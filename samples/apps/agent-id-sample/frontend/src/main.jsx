import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ThunderIDProvider } from "@thunderid/react";
import { BrowserRouter } from "react-router-dom";
import App from "./App.jsx";
import "./styles.css";

const clientId = import.meta.env.VITE_THUNDER_CLIENT_ID;
const baseUrl = import.meta.env.VITE_THUNDER_BASE_URL;
const thunderidReady = Boolean(clientId && baseUrl);

// Scopes requested at sign-in. The trailing `system:*` scopes power the in-app
// Agent Portal: when an admin user signs in, the issued access token carries
// the system permissions needed to call Thunder's /agents and /roles APIs
// directly from the browser. Non-admin users will simply have these scopes
// stripped from the issued token.
const SCOPES = [
  "openid",
  "profile",
  "email",
  "ou",
  "system",
  "system:user",
  "system:group",
  "system:ou:view",
  "system:usertype:view"
];

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <BrowserRouter>
      {thunderidReady ? (
        <ThunderIDProvider
          clientId={clientId}
          baseUrl={baseUrl}
          afterSignInUrl={window.location.origin}
          afterSignOutUrl={window.location.origin}
          scopes={SCOPES}
          discovery={{ wellKnown: { enabled: true } }}
        >
          <App authReady />
        </ThunderIDProvider>
      ) : (
        <App authReady={false} />
      )}
    </BrowserRouter>
  </StrictMode>
);
