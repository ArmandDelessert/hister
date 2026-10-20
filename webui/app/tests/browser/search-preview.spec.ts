// SPDX-License-Identifier: AGPL-3.0-or-later
import { expect, test } from '@playwright/test';

import { mockAPI, original, preview, previewPath } from './fixtures';

test.beforeEach(async ({ page }) => {
  await mockAPI(page);
});

test('narrow results truncate the URL while keeping Preview beside it', async ({
  page,
  isMobile,
}) => {
  const urls = [
    `${original}/${'saved-reading/'.repeat(12)}?lang=en#overview`,
    `${original}/${'reference-notes/'.repeat(12)}`,
  ];
  const label = 'Reference material to read for the next research project';
  await page.routeWebSocket('**/search', (socket) => {
    socket.onMessage(() =>
      socket.send(
        JSON.stringify({
          history: [
            {
              id: 'document-1',
              url: urls[0],
              title: 'Pinned article',
              domain: 'example.com',
              text: 'A saved article with a long label.',
              label,
              pinned: true,
            },
          ],
          documents: [
            {
              id: 'document-2',
              url: urls[1],
              title: 'Another article',
              domain: 'example.com',
              text: 'A saved article without a label.',
            },
          ],
          total: 2,
        }),
      ),
    );
  });
  await page.goto('/?q=article');
  const rows = page.locator('[data-result]');
  await expect(rows).toHaveCount(2);
  await page.evaluate(() => document.fonts.ready);

  for (const width of isMobile ? [375, 320] : [1280, 1024]) {
    await page.setViewportSize({ width, height: 812 });
    for (let index = 0; index < urls.length; index++) {
      const row = rows.nth(index);
      const url = row.locator('.result-url-text');
      const previewLink = row.getByRole('link', { name: 'Preview', exact: true });
      await expect(url).toHaveAttribute('title', urls[index]);
      await expect(previewLink).toBeVisible();
      await expect(async () => {
        const urlBox = (await url.boundingBox())!;
        const previewBox = (await previewLink.boundingBox())!;
        const rowBox = (await row.boundingBox())!;
        expect(
          Math.abs(urlBox.y + urlBox.height / 2 - (previewBox.y + previewBox.height / 2)),
        ).toBeLessThanOrEqual(2);
        expect(previewBox.x).toBeGreaterThanOrEqual(urlBox.x + urlBox.width);
        expect(previewBox.x + previewBox.width).toBeLessThanOrEqual(rowBox.x + rowBox.width);
        expect(urlBox.width).toBeGreaterThan(20);
        expect(await url.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(
          true,
        );
        expect(await url.evaluate((element) => getComputedStyle(element).textOverflow)).toBe(
          'ellipsis',
        );
      }).toPass({ timeout: 5000 });
    }
    await expect(rows.first().getByText(label, { exact: true })).toBeVisible();
    await expect(
      rows.first().getByRole('img', { name: 'Pinned result', exact: true }),
    ).toBeVisible();
    await page.screenshot({ path: test.info().outputPath(`url-preview-${width}.png`) });
  }

  await rows.nth(1).getByRole('link', { name: 'Preview', exact: true }).click();
  await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
});

test('result metadata is available in preview details instead of the result row', async ({
  page,
  isMobile,
}) => {
  await mockAPI(page, { config: { semanticEnabled: true } });
  await page.addInitScript(() => localStorage.setItem('hister-semantic-on', 'true'));
  const updated = 1700086400;
  const articleURL = `${original}?lang=en#overview`;
  const fileURL = 'file:///home/me/notes.txt';
  await page.routeWebSocket('**/search', (socket) => {
    socket.onMessage(() =>
      socket.send(
        JSON.stringify({
          documents: [
            {
              id: 'document-1',
              url: articleURL,
              title: preview.title,
              domain: 'example.com',
              text: 'An indexed article',
              label: 'reading',
              added: preview.added,
              updated,
              add_count: 7,
              score: 0.73,
            },
            {
              id: 'document-2',
              url: fileURL,
              title: 'Local notes',
              domain: '',
              text: 'Saved notes',
              added: preview.added,
              score: 0,
            },
          ],
          total: 2,
        }),
      ),
    );
  });
  await page.route('**/api/preview?**', async (route) => {
    const isFile = new URL(route.request().url()).searchParams.get('url') === fileURL;
    await route.fulfill({
      json: {
        ...preview,
        title: isFile ? 'Local notes' : preview.title,
        updated,
        details: {
          type: isFile ? 'local' : 'web',
          language: 'en',
          label: isFile ? '' : 'reading',
          visits: isFile ? 1 : 7,
          user_id: 1,
          metadata: {},
        },
      },
    });
  });
  await page.goto('/?q=article');
  const rows = page.locator('[data-result]');
  await expect(rows).toHaveCount(2);
  await expect(rows.first().locator('.result-url-text')).toHaveText(
    'example.com/article?lang=en#overview',
  );
  await expect(rows.first().locator('.result-url-text')).toHaveAttribute('title', articleURL);
  await expect(rows.first().getByText('reading', { exact: true })).toBeVisible();
  await expect(rows.first().getByTitle('index count')).toHaveCount(0);
  await expect(rows.first().locator('time')).toHaveCount(0);
  await expect(rows.first().getByText('0.73', { exact: true })).toHaveCount(0);
  await expect(rows.first().getByRole('button', { name: 'Copy URL' })).toHaveCount(0);
  await expect(rows.nth(1).locator('.result-url-text')).toHaveText('/home/me/notes.txt');
  await page.screenshot({ path: test.info().outputPath('result-metadata.png') });

  await rows.first().getByRole('link', { name: 'Preview', exact: true }).click();
  await expect(page.getByRole('button', { name: '1 previous version', exact: true })).toBeVisible();
  await expect(page.locator('time')).toHaveCount(0);
  await page.getByRole('button', { name: 'Show document details', exact: true }).click();
  const details = page.locator('#preview-document-details');
  await expect(details).toBeVisible();
  await expect(details.getByText('0.73', { exact: true })).toBeVisible();
  await expect(
    details.locator('div').filter({ has: page.locator('dt', { hasText: /^visits$/ }) }),
  ).toContainText('7');
  for (const timestamp of [preview.added, updated]) {
    await expect(
      details.locator(`time[datetime="${new Date(timestamp * 1000).toISOString()}"]`),
    ).toBeVisible();
  }

  if (isMobile) await page.getByRole('button', { name: 'Close preview', exact: true }).click();
  await rows.nth(1).getByRole('link', { name: 'Preview', exact: true }).click();
  await expect(details.getByText('0.00', { exact: true })).toBeVisible();
  await expect(details.getByText('0.73', { exact: true })).toHaveCount(0);
  await expect(
    details.locator('div').filter({ has: page.locator('dt', { hasText: /^visits$/ }) }),
  ).toContainText('1');
});

test('recent searches explain browser scope and hiding never deletes server history', async ({
  page,
}) => {
  const mutations: string[] = [];
  page.on('request', (request) => {
    if (request.url().includes('/api/') && request.method() !== 'GET') {
      mutations.push(request.url());
    }
  });
  await page.goto('/');
  const recents = page.getByRole('region', { name: 'Recent searches' });
  await expect(recents).toContainText('Your history remains on the server.');
  await recents.getByRole('button', { name: 'Hide recent search on this browser: svelte' }).click();
  await expect(recents.getByRole('button', { name: 'svelte', exact: true })).toHaveCount(0);
  await expect(recents.getByRole('button', { name: 'golang', exact: true })).toBeVisible();
  await page.reload();
  await expect(recents.getByRole('button', { name: 'golang', exact: true })).toBeVisible();
  await expect(recents.getByRole('button', { name: 'svelte', exact: true })).toHaveCount(0);
  await recents.getByRole('button', { name: 'Hide all on this browser' }).click();
  await expect(recents).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole('link', { name: 'Browse all indexed pages' })).toBeVisible();
  await expect(recents).toHaveCount(0);
  expect(mutations).toEqual([]);
});

for (const failure of ['server', 'network'] as const) {
  test(`a ${failure} preview failure can retry the same archived document`, async ({ page }) => {
    const requests: URLSearchParams[] = [];
    await page.route('**/api/preview?**', async (route) => {
      requests.push(new URL(route.request().url()).searchParams);
      if (requests.length === 1) {
        if (failure === 'network') await route.abort();
        else await route.fulfill({ status: 500, body: 'Internal server details' });
      } else {
        await route.fulfill({
          json: { ...preview, version_id: 7, version_created_at: '2023-11-14T22:13:20Z' },
        });
      }
    });
    await page.goto(previewPath(original, 7));
    await expect(page.getByRole('alert')).toContainText('Could not load this preview.');
    await expect(page.getByRole('link', { name: 'Open original', exact: true })).toHaveAttribute(
      'href',
      original,
    );
    await expect(page.locator('body')).not.toContainText('Internal server details');
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
    await expect(page.getByText('Viewing archived version from', { exact: false })).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
    expect(requests).toHaveLength(2);
    for (const request of requests) {
      expect(request.get('url')).toBe(original);
      expect(request.get('document_id')).toBe('document-1');
      expect(request.get('version')).toBe('7');
    }
  });

  test(`a ${failure} version history failure has a retry instead of an empty list`, async ({
    page,
  }) => {
    let attempts = 0;
    await page.route('**/api/versions?**', async (route) => {
      attempts++;
      if (attempts === 1) {
        if (failure === 'network') await route.abort();
        else await route.fulfill({ status: 500 });
      } else {
        await route.fulfill({
          json: [{ id: 7, created_at: '2023-11-14T22:13:20Z', text_diff: '+Older content' }],
        });
      }
    });
    await page.goto(previewPath());
    await page.getByRole('button', { name: '1 previous version', exact: true }).click();
    await expect(page.getByRole('alert')).toContainText('Could not load previous versions.');
    await page.getByRole('button', { name: 'Retry', exact: true }).click();
    await expect(page.getByRole('button', { name: 'show this version' })).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
    expect(attempts).toBe(2);
  });
}

test('version history distinguishes loading from an empty successful response', async ({
  page,
}) => {
  let finish: () => void = () => {};
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  await page.route('**/api/versions?**', async (route) => {
    await pending;
    await route.fulfill({ json: [] });
  });
  await page.goto(previewPath());
  await page.getByRole('button', { name: '1 previous version', exact: true }).click();
  await expect(page.getByRole('status')).toHaveText('Loading previous versions…');
  finish();
  await expect(page.getByText('No previous versions available.', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: '1 previous version', exact: true }).click();
  await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
});

test('preview errors do not offer unusable or unsafe original links', async ({ page }) => {
  await page.route('**/api/preview?**', (route) => route.fulfill({ status: 500 }));
  for (const url of [
    'remote-file://host/file.txt',
    'file:///private/file.txt',
    'javascript:alert(1)',
  ]) {
    await page.goto(previewPath(url));
    await expect(page.getByRole('alert')).toContainText('Could not load this preview.');
    await expect(page.getByRole('button', { name: 'Retry', exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Open original', exact: true })).toHaveCount(0);
  }
});

test('retry preserves an explicitly selected extractor', async ({ page }) => {
  const requests: URLSearchParams[] = [];
  await page.route('**/api/preview?**', async (route) => {
    requests.push(new URL(route.request().url()).searchParams);
    if (requests.length === 2) await route.fulfill({ status: 500 });
    else await route.fulfill({ json: preview });
  });
  await page.goto(previewPath());
  await page.getByRole('button', { name: 'Change extractor' }).click();
  await page.getByRole('menuitemradio', { name: 'readability', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Could not load this preview.');
  await page.getByRole('button', { name: 'Retry', exact: true }).click();
  await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
  expect(requests).toHaveLength(3);
  expect(requests[1].get('extractor')).toBe('readability');
  expect(requests[2].toString()).toBe(requests[1].toString());
});

test('a delayed preview failure does not replace a newly selected document', async ({ page }) => {
  let finish: () => void = () => {};
  const pending = new Promise<void>((resolve) => {
    finish = resolve;
  });
  await page.route('**/api/preview?**', async (route) => {
    if (new URL(route.request().url()).searchParams.get('url') === original) {
      await pending;
      await route.fulfill({ status: 500 });
    } else {
      await route.fulfill({ json: preview });
    }
  });
  await page.goto(previewPath());
  await expect(page.getByRole('status')).toHaveText('Loading…');
  // The standalone preview uses popstate to select a different document in the same panel.
  await page.evaluate((path) => {
    history.pushState({}, '', path);
    window.dispatchEvent(new PopStateEvent('popstate'));
  }, previewPath('https://example.com/another-article'));
  await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
  finish();
  await page.waitForLoadState('networkidle');
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByText('Saved article content', { exact: true })).toBeVisible();
});
