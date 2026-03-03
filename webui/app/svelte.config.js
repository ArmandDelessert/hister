import adapter from '@sveltejs/adapter-static';

const baseDir = process.env.BASE_PATH || '';

/** @type {import('@sveltejs/kit').Config} */
export default {
  kit: {
    paths: {
      base: '/magic-string-that-we-replace-runtime-in-the-app',
    },
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html'
    })
  }
};
