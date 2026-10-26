import path from "node:path";
import { fileURLToPath } from "node:url";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

const root = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, root, "");
  const proxyTarget = env.VITE_DEV_API_PROXY || env.VITE_EXPERIMENT_API_URL || "http://127.0.0.1:8090";
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
      proxy: {
        "/api": { target: proxyTarget, changeOrigin: true },
      },
    },
    preview: {
      host: true,
      port: 3000,
      proxy: {
        "/api": { target: proxyTarget, changeOrigin: true },
      },
    },
  };
});
