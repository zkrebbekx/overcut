import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// VITE_BASE sets the public path for a sub-path deployment such as GitHub
// Pages (/<repo>/). VITE_STATIC=1 selects the in-browser WebAssembly
// engine instead of the local HTTP API.
export default defineConfig({
  base: process.env.VITE_BASE ?? '/',
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
    },
  },
})
