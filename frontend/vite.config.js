import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// During development the SPA talks to a live Mailfold backend. Point VITE_PROXY
// at a running backend (default: the production instance) so /api and /dav work
// without CORS. In production the SPA is served by the backend itself from the
// same origin, so no proxy is involved.
const target = process.env.VITE_PROXY || 'https://real.mailfold.site';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    strictPort: false,
    proxy: {
      '/api': { target, changeOrigin: true, secure: false },
      '/dav': { target, changeOrigin: true, secure: false },
    },
  },
  build: {
    outDir: 'dist',
    // The distroless backend serves this directory (MAILFOLD_FRONTEND_DIR).
    emptyOutDir: true,
  },
});
