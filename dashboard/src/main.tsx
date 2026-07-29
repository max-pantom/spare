import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/inter/wght.css";
import "@fontsource-variable/geist-mono/wght.css";
import { Agentation } from "agentation";
import { App } from "./App";
import "./styles.css";
import "./dashboard.css";

const showAgentation =
  import.meta.env.DEV &&
  ["desktop-preview", "dashboard-preview"].some((preview) =>
    new URLSearchParams(window.location.search).has(preview)
  );

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
    {showAgentation && <Agentation />}
  </StrictMode>
);
