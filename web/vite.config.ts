import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// The build goes straight into the Go embed folder: `npm run build`, then `go build`.
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: {
    outDir: '../internal/configui/dist',
    emptyOutDir: true,
    target: 'es2022',
  },
  server: {
    // `ROUNDBAR_TOKEN=dev roundbar config --no-open` (listens on 127.0.0.1:7360), then open
    // http://localhost:5173/#t=dev.
    // Without the Go side, http://localhost:5173/?mock runs on built-in sample data.
    proxy: { '/api': { target: 'http://127.0.0.1:7360', changeOrigin: false } },
    // The mock reads ../shared/icons.json
    fs: { allow: ['..'] },
  },
})
