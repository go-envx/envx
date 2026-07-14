import starlight from '@astrojs/starlight'
import { defineConfig } from 'astro/config'

/**
 * Docs site configuration:
 * https://astro.build/config
 *
 * "site" and "base" are intentionally left unset until the deployment target is
 * finalized. GitHub Pages project sites are served from a subpath, so add
 * "site: 'https://go-envx.github.io'" and "base: '/envx'" when wiring up deploy.
 */
export default defineConfig({
  server: {
    host: true,
  },
  integrations: [
    starlight({
      title: 'envx',
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
  ],
})
