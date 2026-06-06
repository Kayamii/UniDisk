import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import { AuthProvider } from "./lib/auth";
import { ConfirmProvider } from "./components/ConfirmDialog";
import { PromptProvider } from "./components/PromptDialog";
import { Toaster } from "./components/ui/sonner";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <ConfirmProvider>
          <PromptProvider>
            <App />
            <Toaster />
          </PromptProvider>
        </ConfirmProvider>
      </AuthProvider>
    </BrowserRouter>
  </React.StrictMode>
);
