// Plugins
import vue from '@vitejs/plugin-vue'
import vuetify, { transformAssetUrls } from 'vite-plugin-vuetify'

// Utilities
import { defineConfig } from 'vite'
import { fileURLToPath, URL } from 'node:url'
import { randomBytes } from 'crypto'

function getUniqueFileName(template) {
  if (template.includes('.js') || template.includes('.css')) {
    const hash = randomBytes(8).toString('hex')
    return template.replace('[name]', hash)
  }
  return template
}

export default defineConfig({
  base: '',
  plugins: [
    vue({
      template: { transformAssetUrls },
    }),
    vuetify({
      autoImport: true,
      styles: {
        configFile: 'src/styles/settings.scss',
      },
    })
  ],
  build: {
    manifest: false,
    outDir: 'dist',
    chunkSizeWarningLimit: 2000,
    rollupOptions: {
      output: {
        inlineDynamicImports: true,
        entryFileNames: getUniqueFileName('assets/[name].js'),
        chunkFileNames: getUniqueFileName('assets/[name].js'),
        assetFileNames: (assetInfo) => {
          if (assetInfo.names.some(name => name.endsWith('.css')))
            return getUniqueFileName('assets/[name].css')
          return 'assets/' + assetInfo.names[0]
        },
      },
    }
  },
  define: { 'process.env': {} },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
    extensions: ['.js', '.json', '.jsx', '.mjs', '.ts', '.tsx', '.vue'],
  },
  server: {
    port: 3000,
    proxy: {
      '/app/api': {
        target: 'http://localhost:2095',
        changeOrigin: true,
      },
    },
  }
})
