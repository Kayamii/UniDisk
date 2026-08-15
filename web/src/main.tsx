import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, HashRouter } from "react-router-dom";
import App from "./App";
import { isDemo } from "./lib/demo";
import { AuthProvider } from "./lib/auth";
import { ConfirmProvider } from "./components/ConfirmDialog";
import { PromptProvider } from "./components/PromptDialog";
import { Toaster } from "./components/ui/sonner";
import "./index.css";

// The demo is served from GitHub Pages, which has no SPA rewrite; hash routing
// keeps deep links and reloads working there. Self-hosted builds use real paths.
const Router = isDemo ? HashRouter : BrowserRouter;

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Router>
      <AuthProvider>
        <ConfirmProvider>
          <PromptProvider>
            <App />
            <Toaster />
          </PromptProvider>
        </ConfirmProvider>
      </AuthProvider>
    </Router>
  </React.StrictMode>
);
