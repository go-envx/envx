import starlight from '@astrojs/starlight'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig } from 'astro/config'
import icon from 'astro-icon'

/**
 * Docs site configuration:
 * https://astro.build/config
 *
 * Served from a GitHub Pages project site at https://go-envx.github.io/envx, so
 * "base" is the repository subpath. When a custom domain is added later, change
 * "site" to that domain and "base" to "/". Internal content links are written
 * relative to the current page so they survive that change without edits.
 */
export default defineConfig({
  site: 'https://go-envx.github.io',
  base: '/envx',
  // Force a trailing slash on every route so relative links always resolve at
  // the correct depth (base included). GitHub Pages already redirects to the
  // trailing-slash form in production; this makes `astro dev` match it instead
  // of serving base-less URLs such as `/envx` where relative links break.
  trailingSlash: 'always',
  server: {
    host: true,
  },
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [
    starlight({
      title: 'envx',
      customCss: ['./src/styles/global.css'],
      description:
        'Layered, composable environment management for multi-project workspaces.',
      logo: {
        src: './src/assets/logo.png',
        alt: 'envx',
      },
      favicon: '/favicon.png',
      social: [
        {
          icon: 'github',
          label: 'GitHub',
          href: 'https://github.com/go-envx/envx',
        },
      ],
      editLink: {
        baseUrl: 'https://github.com/go-envx/envx/edit/main/docs/',
      },
      sidebar: [
        {
          label: 'Guide',
          items: [
            { label: 'Installation', slug: 'guide/installation' },
            { label: 'Getting Started', slug: 'guide/getting-started' },
          ],
        },
        {
          label: 'Configuration',
          items: [{ autogenerate: { directory: 'configuration' } }],
        },
        {
          label: 'Commands',
          items: [{ autogenerate: { directory: 'commands' } }],
        },
        { label: 'Contribute', slug: 'contribute' },
      ],
    }),
    icon(),
  ],
})
