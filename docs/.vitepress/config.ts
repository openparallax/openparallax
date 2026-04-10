import { defineConfig } from 'vitepress'

export default defineConfig({
  appearance: 'dark',
  lang: 'en-US',
  sitemap: {
    hostname: 'https://docs.openparallax.dev',
  },
  markdown: {
    theme: 'night-owl',
    config: (md) => {
      const defaultRender = md.renderer.rules.table_open || function(tokens, idx, options, env, self) {
        return self.renderToken(tokens, idx, options)
      }
      md.renderer.rules.table_open = function(tokens, idx, options, env, self) {
        return '<div class="table-scroll">' + defaultRender(tokens, idx, options, env, self)
      }
      const defaultClose = md.renderer.rules.table_close || function(tokens, idx, options, env, self) {
        return self.renderToken(tokens, idx, options)
      }
      md.renderer.rules.table_close = function(tokens, idx, options, env, self) {
        return defaultClose(tokens, idx, options, env, self) + '</div>'
      }
    },
  },
  title: 'OpenParallax',
  description: 'Open-source personal AI agent with composable security, memory, and messaging modules',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#00dcff' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'OpenParallax' }],
    ['meta', { property: 'og:title', content: 'OpenParallax' }],
    ['meta', { property: 'og:description', content: 'Open-source personal AI agent with composable security, memory, and messaging modules' }],
    ['meta', { property: 'og:image', content: 'https://docs.openparallax.dev/og-image.png' }],
    ['meta', { property: 'og:url', content: 'https://docs.openparallax.dev' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:title', content: 'OpenParallax' }],
    ['meta', { name: 'twitter:description', content: 'Open-source personal AI agent with composable security, memory, and messaging modules' }],
    ['meta', { name: 'twitter:image', content: 'https://docs.openparallax.dev/og-image.png' }],
    ['script', { type: 'application/ld+json' }, JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'SoftwareSourceCode',
      'name': 'OpenParallax',
      'description': 'Open-source personal AI agent with composable security, memory, and messaging modules',
      'url': 'https://docs.openparallax.dev',
      'codeRepository': 'https://github.com/openparallax/openparallax',
      'programmingLanguage': ['Go', 'TypeScript'],
      'license': 'https://www.apache.org/licenses/LICENSE-2.0',
      'applicationCategory': 'DeveloperApplication',
      'operatingSystem': 'Linux, macOS, Windows',
    })],
  ],
  transformPageData(pageData) {
    const canonicalUrl = `https://docs.openparallax.dev/${pageData.relativePath}`
      .replace(/index\.md$/, '')
      .replace(/\.md$/, '')
    pageData.frontmatter.head ??= []
    pageData.frontmatter.head.push(
      ['link', { rel: 'canonical', href: canonicalUrl }],
      ['meta', { property: 'og:url', content: canonicalUrl }],
    )
  },

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'OpenParallax',

    nav: [
      {
        text: 'Documentation',
        items: [
          {
            text: 'Learn',
            items: [
              { text: 'Guide', link: '/guide/' },
              { text: 'Technical', link: '/technical/' },
              { text: 'Evaluation', link: '/eval/' },
            ]
          },
          {
            text: 'Security',
            items: [
              { text: 'Security Architecture', link: '/security/' },
              { text: 'Reference', link: '/reference/config' },
            ]
          },
        ]
      },
      {
        text: 'Modules',
        items: [
          {
            text: 'Security',
            items: [
              { text: 'Shield', link: '/shield/' },
              { text: 'Sandbox', link: '/sandbox/' },
              { text: 'Audit', link: '/audit/' },
            ]
          },
          {
            text: 'Intelligence',
            items: [
              { text: 'Memory', link: '/memory/' },
              { text: 'LLM', link: '/modules/llm' },
              { text: 'Channels', link: '/channels/' },
            ]
          },
          {
            text: 'Infrastructure',
            items: [
              { text: 'Chronicle', link: '/modules/chronicle' },
              { text: 'Crypto', link: '/modules/crypto' },
              { text: 'MCP', link: '/modules/mcp' },
            ]
          },
        ]
      },
      { text: 'Project', link: '/project/changelog' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Overview', link: '/guide/' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Quick Start', link: '/guide/quickstart' },
          ]
        },
        {
          text: 'Using OpenParallax',
          items: [
            { text: 'Configuration', link: '/guide/configuration' },
            { text: 'Hardening', link: '/guide/hardening' },
            { text: 'CLI Commands', link: '/guide/cli' },
            { text: 'Slash Commands', link: '/guide/slash-commands' },
            { text: 'Web UI', link: '/guide/web-ui' },
            { text: 'Sessions & OTR', link: '/guide/sessions' },
            { text: 'Optional Downloads', link: '/guide/optional-downloads' },
          ]
        },
        {
          text: 'Features',
          items: [
            { text: 'Memory', link: '/guide/memory' },
            { text: 'Custom Skills', link: '/guide/skills' },
            { text: 'Tools & Actions', link: '/guide/tools' },
            { text: 'Channels', link: '/guide/channels' },
            { text: 'Security', link: '/guide/security' },
            { text: 'Test Your Own Security', link: '/eval/' },
            { text: 'Heartbeat', link: '/guide/heartbeat' },
          ]
        },
        {
          text: 'Help',
          items: [
            { text: 'Troubleshooting', link: '/guide/troubleshooting' },
          ]
        }
      ],

      '/eval/': [
        {
          text: 'Test Your Own Security',
          items: [
            { text: 'Why an Eval Suite', link: '/eval/' },
            { text: 'Quick Start', link: '/eval/quickstart' },
            { text: 'Test Suite Layout', link: '/eval/test-suite' },
            { text: 'Methodology', link: '/eval/methodology' },
            { text: 'Reproducing Run-013', link: '/eval/reproducing' },
            { text: 'Run History', link: '/eval/runs' },
            { text: 'Reports', link: '/eval/reports' },
            { text: 'Adding Test Cases', link: '/eval/contributing-tests' },
          ]
        }
      ],

      '/technical/': [
        {
          text: 'Architecture',
          items: [
            { text: 'Overview', link: '/technical/' },
            { text: 'Process Model', link: '/technical/process-model' },
            { text: 'Message Pipeline', link: '/technical/pipeline' },
            { text: 'The Ecosystem', link: '/technical/ecosystem' },
          ]
        },
        {
          text: 'Design Philosophy',
          items: [
            { text: 'Defense in Depth', link: '/technical/design-security' },
            { text: 'Process Isolation', link: '/technical/design-isolation' },
            { text: 'Token Economy', link: '/technical/design-efficiency' },
            { text: 'Modularity', link: '/technical/design-modularity' },
          ]
        },
        {
          text: 'Core Systems',
          items: [
            { text: 'Engine', link: '/technical/engine' },
            { text: 'Agent', link: '/technical/agent' },
            { text: 'gRPC Services', link: '/technical/grpc' },
            { text: 'Event System', link: '/technical/event-system' },
          ]
        },
        {
          text: 'Security',
          items: [
            { text: 'File Protection', link: '/technical/protection' },
            { text: 'Cryptographic Primitives', link: '/technical/crypto' },
          ]
        },
        {
          text: 'Integration',
          items: [
            { text: 'Web Server', link: '/technical/web-server' },
            { text: 'Extending OpenParallax', link: '/technical/extending' },
          ]
        },
      ],

      '/shield/': [
        {
          text: 'Shield',
          items: [
            { text: 'Overview', link: '/shield/' },
            { text: 'Quick Start', link: '/shield/quickstart' },
          ]
        },
        {
          text: 'Security Tiers',
          items: [
            { text: 'Tier 0 — Policy', link: '/shield/tier0' },
            { text: 'Tier 1 — Classifier', link: '/shield/tier1' },
            { text: 'Tier 2 — LLM Evaluator', link: '/shield/tier2' },
            { text: 'Tier 3 — Human Approval', link: '/shield/tier3' },
            { text: 'ONNX Classifier', link: '/shield/classifier' },
          ]
        },
        {
          text: 'Usage',
          items: [
            { text: 'Go Library', link: '/shield/go' },
            { text: 'Python', link: '/shield/python' },
            { text: 'Node.js', link: '/shield/node' },
            { text: 'Standalone Proxy', link: '/shield/standalone' },
            { text: 'MCP Gateway', link: '/shield/mcp-proxy' },
          ]
        },
        {
          text: 'Reference',
          items: [
            { text: 'Policy Syntax', link: '/shield/policies' },
            { text: 'Configuration', link: '/shield/configuration' },
          ]
        }
      ],

      '/memory/': [
        {
          text: 'Memory',
          items: [
            { text: 'Overview', link: '/memory/' },
            { text: 'Quick Start', link: '/memory/quickstart' },
          ]
        },
        {
          text: 'Backends',
          items: [
            { text: 'Choosing a Backend', link: '/memory/backends/' },
            { text: 'SQLite', link: '/memory/backends/sqlite' },
            { text: 'PostgreSQL + pgvector', link: '/memory/backends/pgvector' },
            { text: 'Qdrant', link: '/memory/backends/qdrant' },
            { text: 'Pinecone', link: '/memory/backends/pinecone' },
            { text: 'Weaviate', link: '/memory/backends/weaviate' },
            { text: 'ChromaDB', link: '/memory/backends/chroma' },
            { text: 'Redis', link: '/memory/backends/redis' },
          ]
        },
        {
          text: 'Usage',
          items: [
            { text: 'Go Library', link: '/memory/go' },
            { text: 'Python', link: '/memory/python' },
            { text: 'Node.js', link: '/memory/node' },
            { text: 'Embeddings', link: '/memory/embeddings' },
          ]
        }
      ],

      '/audit/': [
        {
          text: 'Audit',
          items: [
            { text: 'Overview', link: '/audit/' },
            { text: 'Quick Start', link: '/audit/quickstart' },
            { text: 'Go Library', link: '/audit/go' },
            { text: 'Python', link: '/audit/python' },
            { text: 'Node.js', link: '/audit/node' },
            { text: 'Hash Chain Verification', link: '/audit/verification' },
          ]
        }
      ],

      '/sandbox/': [
        {
          text: 'Sandbox',
          items: [
            { text: 'Overview', link: '/sandbox/' },
            { text: 'Quick Start', link: '/sandbox/quickstart' },
            { text: 'Go Library', link: '/sandbox/go' },
          ]
        },
        {
          text: 'Platforms',
          items: [
            { text: 'Linux — Landlock', link: '/sandbox/landlock' },
            { text: 'macOS — sandbox-exec', link: '/sandbox/sandbox-exec' },
            { text: 'Windows — Job Objects', link: '/sandbox/job-objects' },
            { text: 'Canary Probes', link: '/sandbox/canary' },
          ]
        }
      ],

      '/channels/': [
        {
          text: 'Channels',
          items: [
            { text: 'Overview', link: '/channels/' },
            { text: 'Quick Start', link: '/channels/quickstart' },
            { text: 'Go Library', link: '/channels/go' },
            { text: 'Python', link: '/channels/python' },
            { text: 'Node.js', link: '/channels/node' },
          ]
        },
        {
          text: 'Adapters',
          items: [
            { text: 'WhatsApp', link: '/channels/whatsapp' },
            { text: 'Telegram', link: '/channels/telegram' },
            { text: 'Discord', link: '/channels/discord' },
            { text: 'Slack', link: '/channels/slack' },
            { text: 'Signal', link: '/channels/signal' },
            { text: 'Teams', link: '/channels/teams' },
            { text: 'iMessage', link: '/channels/imessage' },
          ]
        }
      ],

      '/security/': [
        {
          text: 'Security Architecture',
          items: [
            { text: 'Overview', link: '/security/' },
          ]
        },
        {
          text: 'Defense Mechanisms',
          items: [
            { text: 'Structural Isolation', link: '/security/structural-isolation' },
            { text: 'Action Validation', link: '/security/action-validation' },
            { text: 'State Protection', link: '/security/state-protection' },
            { text: 'Input/Output Safety', link: '/security/input-output-safety' },
            { text: 'Resource Limiting', link: '/security/resource-limiting' },
            { text: 'Cryptographic Foundations', link: '/security/cryptographic-foundations' },
          ]
        },
        {
          text: 'Policy & Configuration',
          items: [
            { text: 'Threat Model', link: '/security/threat-model' },
            { text: 'Non-Negotiable Defenses', link: '/security/non-negotiable' },
            { text: 'Hardening', link: '/security/hardening' },
            { text: 'Information Flow Control', link: '/security/ifc' },
          ]
        },
      ],

      '/modules/': [
        {
          text: 'Other Modules',
          items: [
            { text: 'Chronicle', link: '/modules/chronicle' },
            { text: 'LLM Providers', link: '/modules/llm' },
            { text: 'Crypto Primitives', link: '/modules/crypto' },
            { text: 'MCP Client', link: '/modules/mcp' },
          ]
        }
      ],

      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'Configuration', link: '/reference/config' },
            { text: 'Environment Variables', link: '/reference/env-vars' },
            { text: 'Event Types', link: '/reference/events' },
            { text: 'Action Types', link: '/reference/actions' },
            { text: 'gRPC API', link: '/reference/grpc-api' },
            { text: 'REST API', link: '/reference/rest-api' },
            { text: 'WebSocket Protocol', link: '/reference/websocket' },
            { text: 'Policy Syntax', link: '/reference/policy-syntax' },
          ]
        }
      ],

      '/project/': [
        {
          text: 'Project',
          items: [
            { text: 'Changelog', link: '/project/changelog' },
            { text: 'Roadmap', link: '/project/roadmap' },
          ]
        }
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/openparallax/openparallax' },
    ],

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Released under the <a href="https://github.com/openparallax/openparallax/blob/main/LICENSE">Apache License 2.0</a>.',
      copyright: '\u00A9 2026\u2013present OpenParallax Contributors',
    },

    editLink: {
      pattern: 'https://github.com/openparallax/openparallax/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
  },
})
