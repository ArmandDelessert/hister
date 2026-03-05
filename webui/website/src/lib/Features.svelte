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
      { icon: Shield, title: 'PRIVACY-FOCUSED', desc: 'Keep your browsing history indexed locally. No remote servers, no tracking, no data leaving your machine.', bg: 'bg-brutal-card', textColor: 'text-[var(--text-primary)]', descColor: 'text-[var(--text-secondary)]', iconBg: 'bg-hister-indigo' },
      { icon: FileText, title: 'FULL-TEXT INDEXING', desc: "Search through the actual content of web pages you've visited, not just URLs and titles.", bg: 'bg-brutal-card', textColor: 'text-[var(--text-primary)]', descColor: 'text-[var(--text-secondary)]', iconBg: 'bg-hister-coral' },
    ],
    [
      { icon: Zap, title: 'BLAZING FAST', desc: 'Instant results as you type. Full keyboard-driven workflow for maximum efficiency.', bg: 'bg-hister-indigo', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
      { icon: Search, title: 'ADVANCED SEARCH', desc: 'Powerful query language for precise results.', bg: 'bg-[var(--text-primary)]', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
      { icon: Settings, title: 'FLEXIBLE MANAGEMENT', desc: 'Configure blacklist and priority rules to control your indexed content.', bg: 'bg-hister-teal', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
    ],
    [
      { icon: Puzzle, title: 'BROWSER EXTENSIONS', desc: 'Available for Chrome and Firefox.', bg: 'bg-hister-lime', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
      { icon: WifiOff, title: 'OFFLINE PREVIEW', desc: 'Readable offline previews of any result. Access your saved pages without internet.', bg: 'bg-hister-coral', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
      { icon: Download, title: 'BROWSER IMPORT', desc: 'Import existing Firefox or Chrome history with a single command.', bg: 'bg-hister-rose', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-white/20' },
    ],
  ];

  const aliasCard = {
    icon: Terminal, title: 'SEARCH ALIASES', desc: 'Create custom keyword shortcuts to simplify complex or frequently used queries.', bg: 'bg-hister-cyan', textColor: 'text-white', descColor: 'text-white/80', iconBg: 'bg-[#f5f5f5]', iconColor: 'text-hister-teal',
  };
</script>

<section id="features" class="max-w-[2000px] mx-auto bg-brutal-bg px-6 md:px-12 py-12 md:py-16 flex flex-col gap-10">
  <PageHeader color="hister-coral" size="lg" tag="h2">FEATURES</PageHeader>

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
