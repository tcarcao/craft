import { defineConfig } from 'vitepress'
import { readFileSync } from 'fs'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'

const __dirname = dirname(fileURLToPath(import.meta.url))

// Load simple working Craft grammar from JSON
const craftGrammarRaw = readFileSync(join(__dirname, 'craft-simple.json'), 'utf-8')
const craftGrammar = JSON.parse(craftGrammarRaw)

// Load custom theme with VSCode extension colors
const craftThemeRaw = readFileSync(join(__dirname, 'craft-theme.json'), 'utf-8')
const craftTheme = JSON.parse(craftThemeRaw)

export default defineConfig({
  title: "Craft Language",
  description: "A DSL for modeling business use cases and domain interactions",
  base: '/craft/',

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Home', link: '/' },
      { text: 'Guide', link: '/guide/introduction' },
      { text: 'Language', link: '/language/overview' },
      { text: 'Extension', link: '/extension/installation' },
      { text: 'Examples', link: '/examples/ecommerce' }
    ],

    sidebar: [
      {
        text: 'Getting Started',
        items: [
          { text: 'Introduction', link: '/guide/introduction' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Quick Start', link: '/guide/quickstart' }
        ]
      },
      {
        text: 'Language Reference',
        items: [
          { text: 'Overview', link: '/language/overview' },
          { text: 'Domains', link: '/language/domains' },
          { text: 'Actors', link: '/language/actors' },
          { text: 'Services', link: '/language/services' },
          { text: 'Use Cases', link: '/language/use-cases' },
          { text: 'Architecture', link: '/language/architecture' },
          { text: 'Exposures', link: '/language/exposures' }
        ]
      },
      {
        text: 'VSCode Extension',
        items: [
          { text: 'Installation', link: '/extension/installation' },
          { text: 'Features', link: '/extension/features' },
          { text: 'Commands', link: '/extension/commands' },
          { text: 'Configuration', link: '/extension/configuration' }
        ]
      },
      {
        text: 'Examples',
        items: [
          { text: 'E-commerce', link: '/examples/ecommerce' },
          { text: 'Banking System', link: '/examples/banking' },
          { text: 'User Management', link: '/examples/user-management' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/tcarcao/craft' },
      { icon: 'github', link: 'https://github.com/tcarcao/craft-vscode-extension' }
    ],

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2025 Tiago Carcao'
    },

    search: {
      provider: 'local'
    }
  },

  markdown: {
    theme: craftTheme,
    languages: [craftGrammar]
  }
})
