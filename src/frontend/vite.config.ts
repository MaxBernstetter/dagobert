import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, root, "");
  // Connect services (ExperimentService, ProviderService, BackfillService) on the Floci API Gateway. scripts/local-api-url.sh
  // sets VITE_DEV_API_PROXY (origin) and VITE_DEV_API_PREFIX (/_aws/execute-api/<id>/$default); without a prefix the path is
  // forwarded unchanged.
  const proxyTarget = env.VITE_DEV_API_PROXY || env.VITE_EXPERIMENT_API_URL || "http://127.0.0.1:11566";
  const apiPrefix = (env.VITE_DEV_API_PREFIX ?? "").replace(/\/$/, "");
  const proxy = {
    "/tradingcopilot.v1.": {
      target: proxyTarget,
      changeOrigin: true,
      ...(apiPrefix ? { rewrite: (p: string) => apiPrefix + p } : {}),
    },
  };
  return {
    plugins: [react(), tailwindcss()],
    publicDir: path.resolve(root, "public"),
    resolve: {
      dedupe: ["react", "react-dom"],
      alias: {
        "@": path.resolve(root, "src"),
        react: path.resolve(root, "node_modules/react"),
        "react-dom": path.resolve(root, "node_modules/react-dom"),
      },
    },
    server: {
      host: true,
      port: 3000,
      fs: { allow: [path.resolve(root, "../..")] },
      proxy,
    },
    preview: {
      host: true,
      port: 3000,
      proxy,
    },
  };
});
