interface DocsCategory {
  name: string;
  slugs: string[];
  color: string;
}

export const docsStructure: DocsCategory[] = [
  {
    name: 'Getting Started',
    slugs: ['intro', 'installing', 'quickstart', 'browser-extension', 'troubleshooting'],
    color: 'indigo',
  },
  {
    name: 'Using in the Terminal',
    slugs: ['terminal-client', 'importing-browser-history'],
    color: 'lime',
  },
  {
    name: 'Reference',
    slugs: ['configuration', 'query-language', 'extractors', 'mcp'],
    color: 'teal',
  },
  {
    name: 'Advanced Server Setup',
    slugs: ['server-setup', 'docker', 'user-handling'],
    color: 'coral',
  },
];
