<script lang="ts">
  import Shield from '@lucide/svelte/icons/shield';
  import FileText from '@lucide/svelte/icons/file-text';
  import Zap from '@lucide/svelte/icons/zap';
  import Search from '@lucide/svelte/icons/search';
  import Settings from '@lucide/svelte/icons/settings';
  import Puzzle from '@lucide/svelte/icons/puzzle';
  import WifiOff from '@lucide/svelte/icons/wifi-off';
  import Download from '@lucide/svelte/icons/download';
  import Terminal from '@lucide/svelte/icons/terminal';
  import { PageHeader } from '@hister/components';
  import { FeatureCard } from '@hister/components';

  const rows = [
    [
      {
        icon: Shield,
        title: 'Privacy-focused',
        desc: 'Keep your browsing history indexed locally. No remote servers, no tracking, no data leaving your machine.',
        bg: 'bg-brutal-card',
        textColor: 'text-[var(--text-primary)]',
        descColor: 'text-[var(--text-secondary)]',
        iconBg: 'bg-hister-indigo',
      },
      {
        icon: FileText,
        title: 'Full-text Indexing',
        desc: 'Search through the actual content of web pages and your local files.',
        bg: 'bg-brutal-card',
        textColor: 'text-[var(--text-primary)]',
        descColor: 'text-[var(--text-secondary)]',
        iconBg: 'bg-hister-coral',
      },
    ],
    [
      {
        icon: Zap,
        title: 'Blazing Fast',
        desc: 'Instant results as you type. Full keyboard-driven workflow for maximum efficiency.',
        bg: 'bg-hister-indigo',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
      {
        icon: Search,
        title: 'Advanced Search',
        desc: 'Powerful query language for precise results.',
        bg: 'bg-[var(--text-primary)]',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
      {
        icon: Settings,
        title: 'Flexible Management',
        desc: 'Configure blacklist and priority rules to control your indexed content.',
        bg: 'bg-hister-teal',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
    ],
    [
      {
        icon: Puzzle,
        title: 'Browser Extensions',
        desc: 'Available for <a class="underline" href="https://chromewebstore.google.com/detail/hister/cciilamhchpmbdnniabclekddabkifhb">Chrome</a> and <a class="underline" href="https://addons.mozilla.org/en-US/firefox/addon/hister/">Firefox</a> to automatically index websites you visit.',
        bg: 'bg-hister-lime',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
      {
        icon: WifiOff,
        title: 'Offline Preview',
        desc: 'Readable offline previews of any result. Access your saved pages without internet.',
        bg: 'bg-hister-coral',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
      {
        icon: Download,
        title: 'Browser Import',
        desc: 'Import existing Firefox or Chrome history with a single command.',
        bg: 'bg-hister-rose',
        textColor: 'text-white',
        descColor: 'text-white/80',
        iconBg: 'bg-white/20',
      },
    ],
  ];

  const aliasCard = {
    icon: Terminal,
    title: 'Search Aliases',
    desc: 'Create custom keyword shortcuts to simplify complex or frequently used queries.',
    bg: 'bg-hister-cyan',
    textColor: 'text-white',
    descColor: 'text-white/80',
    iconBg: 'bg-[#f5f5f5]',
    iconColor: 'text-hister-teal',
  };
</script>

<section
  id="features"
  class="bg-brutal-bg mx-auto flex max-w-[2000px] flex-col gap-10 px-6 py-12 md:px-12 md:py-16"
>
  <PageHeader color="hister-coral" size="lg" tag="h2">Features</PageHeader>

  <div class="flex flex-col gap-6">
    {#each rows as row}
      <div class="grid grid-cols-1 gap-4 {row.length === 2 ? 'md:grid-cols-2' : 'md:grid-cols-3'}">
        {#each row as card}
          <FeatureCard
            icon={card.icon}
            title={card.title}
            description={card.desc}
            bg={card.bg}
            textColor={card.textColor}
            descColor={card.descColor}
            iconBg={card.iconBg}
          />
        {/each}
      </div>
    {/each}

    <FeatureCard
      icon={aliasCard.icon}
      title={aliasCard.title}
      description={aliasCard.desc}
      bg={aliasCard.bg}
      textColor={aliasCard.textColor}
      descColor={aliasCard.descColor}
      iconBg={aliasCard.iconBg}
      iconColor={aliasCard.iconColor}
      horizontal
    />
  </div>
</section>
