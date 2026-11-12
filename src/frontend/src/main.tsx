import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { LoginGate } from "@/components/login-gate";
import "./globals.css";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");
createRoot(root).render(
  <StrictMode>
    <LoginGate>
      <App />
    </LoginGate>
  </StrictMode>,
);
