// SPDX-License-Identifier: AGPL-3.0-or-later

import { onMount, tick, untrack } from 'svelte';
import { resolve } from '$app/paths';

const previewDetailsOpenStorageKey = 'hister-preview-details-open';

/** Builds the preview page URL for a document. */
export function buildPreviewUrl(
  id: string,
  title: string,
  versionId?: number | null,
  documentId?: string,
): string {
  const versionParam = versionId != null ? `&version=${versionId}` : '';
  const documentParam = documentId ? `&document_id=${encodeURIComponent(documentId)}` : '';
  return resolve(
    `preview?id=${encodeURIComponent(id)}${title ? '&title=' + encodeURIComponent(title) : ''}${versionParam}${documentParam}`,
  );
}

/** Pushes a preview entry onto the browser history stack. */
export function pushPreviewHistory(
  id: string,
  title: string,
  versionId?: number | null,
  documentId?: string,
) {
  history.pushState(
    { type: 'preview', id, title, versionId: versionId ?? null, documentId: documentId ?? '' },
    '',
    buildPreviewUrl(id, title, versionId, documentId),
  );
}

/** Replaces the current browser history entry with a preview entry. */
export function replacePreviewHistory(
  id: string,
  title: string,
  versionId?: number | null,
  documentId?: string,
) {
  history.replaceState(
    { type: 'preview', id, title, versionId: versionId ?? null, documentId: documentId ?? '' },
    '',
    buildPreviewUrl(id, title, versionId, documentId),
  );
}

/** Returns whether document details should be expanded in preview panels. */
export function getStoredPreviewDetailsOpen(): boolean {
  return localStorage.getItem(previewDetailsOpenStorageKey) === 'true';
}

/** Persists the document details disclosure preference across preview panels. */
export function setStoredPreviewDetailsOpen(open: boolean): void {
  localStorage.setItem(previewDetailsOpenStorageKey, String(open));
}

/** Owns preview navigation and panel preferences for a result page. */
export class PreviewState {
  url = $state('');
  documentId = $state('');
  title = $state('');
  versionId = $state<number | null>(null);
  fullscreen = $state(false);
  isDesktop = $state(false);
  panelOpen = $state(true);
  widthPct = $state(parseFloat(localStorage.getItem('hister-panel-width') ?? '') || 50);
  container = $state<HTMLElement>();
  skipUrl = false;

  constructor(
    private options: {
      storageKey: string;
      onReturn: () => void;
      beforeOpen?: () => void;
    },
  ) {
    this.panelOpen = localStorage.getItem(options.storageKey) !== 'false';
    onMount(() => {
      const mq = window.matchMedia('(min-width: 1280px)');
      this.isDesktop = mq.matches;
      const handler = (event: MediaQueryListEvent) => {
        this.isDesktop = event.matches;
      };
      mq.addEventListener('change', handler);
      return () => mq.removeEventListener('change', handler);
    });
  }

  setPanelOpen(open: boolean) {
    this.panelOpen = open;
    localStorage.setItem(this.options.storageKey, String(open));
  }

  private select(url: string, title: string, documentId: string) {
    this.url = url;
    this.title = title;
    this.documentId = documentId;
    this.versionId = null;
  }

  open(url: string, title: string, documentId = '') {
    this.select(url, title, documentId);
    if (this.isDesktop) {
      this.options.beforeOpen?.();
      this.setPanelOpen(true);
    } else {
      this.enterFullscreen();
    }
  }

  enterFullscreen = () => {
    this.options.beforeOpen?.();
    this.fullscreen = true;
    this.writeHistory(pushPreviewHistory);
  };

  exitFullscreen = () => {
    this.fullscreen = false;
    this.withSkipUrl(this.options.onReturn);
  };

  close = () => {
    this.setPanelOpen(false);
    if (this.fullscreen) this.exitFullscreen();
  };

  toggle(openSelection: () => void) {
    if (this.fullscreen) {
      if (this.isDesktop) this.exitFullscreen();
      else this.close();
    } else if (this.isDesktop && this.panelOpen) {
      this.enterFullscreen();
    } else {
      openSelection();
    }
  }

  syncSelection(url: string, title: string, documentId = '') {
    const fullscreen = this.fullscreen;
    if (!this.isDesktop || (!this.panelOpen && !fullscreen)) return;
    // Selection effects track the result and panel visibility, not their own writes.
    untrack(() => {
      if (this.url === url && this.documentId === documentId) return;
      this.select(url, title, documentId);
      if (fullscreen && url) this.writeHistory(replacePreviewHistory);
    });
  }

  setVersion = (id: number | null) => {
    this.versionId = id;
    if (this.fullscreen) this.writeHistory(replacePreviewHistory);
  };

  restore(event: PopStateEvent): boolean {
    const state = event.state as {
      type?: string;
      id?: string;
      documentId?: string;
      title?: string;
      versionId?: number | null;
    } | null;
    this.fullscreen = state?.type === 'preview';
    if (!this.fullscreen) return false;
    this.select(state?.id || '', state?.title || '', state?.documentId || '');
    this.panelOpen = true;
    this.versionId = state?.versionId ?? null;
    return true;
  }

  withSkipUrl(fn: () => void) {
    this.skipUrl = true;
    fn();
    tick().then(() => {
      this.skipUrl = false;
    });
  }

  private writeHistory(write: typeof pushPreviewHistory) {
    this.withSkipUrl(() => write(this.url, this.title, this.versionId, this.documentId));
  }

  startResize = (e: MouseEvent) => {
    e.preventDefault();
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    let lastPct = 50;
    const onMove = (e: MouseEvent) => {
      const container = this.container;
      if (!container) return;
      const rect = container.getBoundingClientRect();
      const fromRight = rect.right - e.clientX;
      lastPct = Math.min(85, Math.max(15, (fromRight / rect.width) * 100));
      this.widthPct = lastPct;
    };
    const onUp = () => {
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      localStorage.setItem('hister-panel-width', String(lastPct));
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    };
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  };
}
