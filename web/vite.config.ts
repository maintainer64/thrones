import { defineConfig } from "vite";
import { unlink } from "node:fs/promises";
import orderBases from "./src/artwork/order-bases.json";
const backend = process.env.VITE_BACKEND_TARGET || "http://127.0.0.1:18081";
export default defineConfig({
  plugins: [
    {
      name: "webp-order-assets-only",
      apply: "build",
      async closeBundle() {
        // Remove only Vite's generated copies. Keep all five editable PNG sources
        // in public/, but ship their lighter WebP renditions in the web bundle.
        for (const filename of Object.values(orderBases)) {
          await unlink(
            new URL(`./dist/media/orders/${filename}`, import.meta.url),
          ).catch((error) => {
            if (error.code !== "ENOENT") throw error;
          });
        }
      },
    },
  ],
  server: {
    port: 5174,
    proxy: {
      "/api": { target: backend, ws: true },
      "/media": { target: backend },
      "/local-assets": { target: backend },
    },
  },
  build: { outDir: "dist" },
});
