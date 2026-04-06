import { defineConfig } from 'vitepress'

export default defineConfig({
  appearance: 'dark',
  markdown: {
    theme: 'night-owl',
  },
  title: 'OpenParallax',
  description: 'Open-source personal AI agent with composable security, memory, and messaging modules',
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#00dcff' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'OpenParallax' }],
    ['meta', { property: 'og:description', content: 'Open-source personal AI agent with composable security, memory, and messaging modules' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'OpenParallax',

    nav: [
      { text: 'Guide', link: '/guide/' },
      { text: 'Technical', link: '/technical/' },
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
              { text: 'IFC', link: '/modules/ifc' },
              { text: 'Crypto', link: '/modules/crypto' },
              { text: 'MCP', link: '/modules/mcp' },
            ]
          },
        ]
      },
      { text: 'Reference', link: '/reference/config' },
      { text: 'Changelog', link: '/changelog' },
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
            { text: 'CLI Commands', link: '/guide/cli' },
            { text: 'Web UI', link: '/guide/web-ui' },
            { text: 'Sessions & OTR', link: '/guide/sessions' },
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

      '/modules/': [
        {
          text: 'Other Modules',
          items: [
            { text: 'Chronicle', link: '/modules/chronicle' },
            { text: 'LLM Providers', link: '/modules/llm' },
            { text: 'Information Flow Control', link: '/modules/ifc' },
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
