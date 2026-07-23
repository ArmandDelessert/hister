import { defineConfig } from 'vite';
import { resolve } from 'path';
import { readFileSync } from 'fs';

const pkg = JSON.parse(readFileSync(resolve(import.meta.dirname, 'package.json'), 'utf-8'));
const userscriptHeader = `// ==UserScript==
// @name         Hister for qutebrowser
// @namespace    https://github.com/asciimoo/hister
// @version      ${pkg.version}
// @description  Automatically index rendered pages in Hister
// @match        http://*/*
// @match        https://*/*
// @run-at       document-idle
// @qute-js-world user
// @grant        none
// @noframes
// ==/UserScript==

// Edit these values after copying the script into qutebrowser.
const HISTER_QUTEBROWSER_CONFIG = Object.freeze({
  serverURL: 'http://127.0.0.1:4433/',
  accessToken: 'replace-with-app-access-token',
  label: '',
});
`;

export default defineConfig({
  build: {
    outDir: resolve(import.meta.dirname, '../../scripts'),
    emptyOutDir: false,
    minify: false,
    sourcemap: false,
    rolldownOptions: {
      input: resolve(import.meta.dirname, 'src/qutebrowser/qutebrowser.ts'),
      output: {
        format: 'iife',
        entryFileNames: 'hister-qutebrowser.user.js',
        banner: userscriptHeader,
      },
    },
  },
});
