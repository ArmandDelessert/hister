export interface DocsCategory {
	name: string;
	slugs: string[];
}

export const docsStructure: DocsCategory[] = [
	{
		name: "Getting Started",
		slugs: ["intro", "installing", "getting-started"],
	},
	{
		name: "Reference",
		slugs: ["configuration", "query-language"],
	},
	{
		name: "Deployment",
		slugs: ["server-setup"],
	},
];
