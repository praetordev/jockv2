import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import electron from 'vite-plugin-electron';
import renderer from 'vite-plugin-electron-renderer';
import path from 'path';
import {defineConfig, loadEnv} from 'vite';

export default defineConfig(({mode}) => {
  const env = loadEnv(mode, '.', '');
  return {
    test: {
      environment: 'jsdom',
      globals: true,
      setupFiles: './src/test-setup.ts',
    },
    plugins: [
      react(),
      tailwindcss(),
      electron([
        {
          entry: 'electron/main.ts',
          onstart({ startup }) {
            startup(['.', '--no-sandbox'], {
              env: { ...process.env, ELECTRON_RUN_AS_NODE: '' },
            });
          },
          vite: {
            build: {
              rollupOptions: {
                external: ['node-pty'],
              },
            },
          },
        },
        {
          entry: 'electron/preload.ts',
          onstart({ reload }) {
            reload();
          },
        },
      ]),
      renderer(),
    ],
    define: {
      'process.env.GEMINI_API_KEY': JSON.stringify(env.GEMINI_API_KEY),
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, '.'),
      },
    },
    server: {
      hmr: process.env.DISABLE_HMR !== 'true',
    },
  };
});
