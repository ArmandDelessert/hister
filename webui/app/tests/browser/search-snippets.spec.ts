import { expect, test, type Locator, type Page } from '@playwright/test';

import { mockAPI, original } from './fixtures';

const context = 'Surrounding search result context. '.repeat(25);
const longText = `<mark>needle</mark> ${context} End of the excerpt.`;

async function mockSnippets(page: Page, texts: string[]) {
  await page.routeWebSocket('**/search', (socket) => {
    socket.onMessage(() =>
      socket.send(
        JSON.stringify({
          documents: texts.map((text, index) => ({
            id: `document-${index}`,
            url: `${original}/${index}`,
            title: `Article ${index + 1}`,
            domain: 'example.com',
            text,
            added: 1700000000,
          })),
          total: texts.length,
        }),
      ),
    );
  });
}

async function expectClipped(snippet: Locator, clipped: boolean) {
  await expect
    .poll(() => snippet.evaluate((element) => element.scrollHeight > element.clientHeight + 1))
    .toBe(clipped);
  if (clipped) {
    expect(
      await snippet.evaluate(
        (element) => element.clientHeight / parseFloat(getComputedStyle(element).lineHeight),
      ),
    ).toBeCloseTo(3, 1);
  }
}

test.beforeEach(async ({ page }) => {
  await mockAPI(page, { canWrite: false });
});

test('long snippets expand individually with keyboard access and preserve their highlights', async ({
  page,
}) => {
  await mockSnippets(page, [longText, longText, 'A short excerpt.', '']);
  await page.goto('/?q=needle');
  const rows = page.locator('[data-result]');
  await expect(rows).toHaveCount(4);
  await page.evaluate(() => document.fonts.ready);
  const snippet = rows.first().locator('.result-snippet');
  await expect(snippet).toContainText(context.trim());
  await expect(snippet.locator('mark')).toHaveText('needle');
  await expectClipped(snippet, true);
  await expectClipped(rows.nth(1).locator('.result-snippet'), true);
  await expect(rows.nth(2).getByRole('button', { name: 'Show more', exact: true })).toHaveCount(0);
  await expect(rows.nth(3).locator('.result-snippet')).toHaveCount(0);
  await page.screenshot({ path: test.info().outputPath('compact-snippets.png') });

  const expand = rows.first().getByRole('button', { name: 'Show more', exact: true });
  await expect(expand).toHaveAttribute('aria-expanded', 'false');
  await expect(expand).toHaveAttribute('aria-controls', (await snippet.getAttribute('id'))!);
  await expand.focus();
  await expand.press('Enter');
  await expect(page).toHaveURL(/q=needle/);
  const collapse = rows.first().getByRole('button', { name: 'Show less', exact: true });
  await expect(collapse).toBeFocused();
  await expect(collapse).toHaveAttribute('aria-expanded', 'true');
  await expectClipped(snippet, false);
  await expectClipped(rows.nth(1).locator('.result-snippet'), true);
  await expect(snippet.locator('mark')).toHaveText('needle');
  await collapse.press('Space');
  await expectClipped(snippet, true);
  await expect(expand).toBeFocused();
});

test('the expanded snippet preference persists and can return to compact snippets', async ({
  page,
}) => {
  await mockSnippets(page, [longText, longText]);
  await page.goto('/?q=needle');
  const snippets = page.locator('.result-snippet');
  await expect(snippets).toHaveCount(2);
  await expectClipped(snippets.first(), true);

  const actions = page.getByRole('button', { name: 'Actions', exact: true });
  const preference = page.getByRole('checkbox', { name: 'Always expand snippets', exact: true });
  await actions.click();
  await preference.check();
  await page.keyboard.press('Escape');
  for (const snippet of await snippets.all()) await expectClipped(snippet, false);

  await page.locator('[data-result]').first().getByRole('button', { name: 'Show less' }).click();
  await expectClipped(snippets.first(), true);
  await expectClipped(snippets.nth(1), false);
  await page.reload();
  await expect(snippets).toHaveCount(2);
  for (const snippet of await snippets.all()) await expectClipped(snippet, false);
  await actions.click();
  await expect(preference).toBeChecked();
  await preference.uncheck();
  await page.keyboard.press('Escape');
  for (const snippet of await snippets.all()) await expectClipped(snippet, true);
  await page.reload();
  await expect(snippets).toHaveCount(2);
  for (const snippet of await snippets.all()) await expectClipped(snippet, true);
});

test('snippet controls follow available width and updated search text', async ({ page }) => {
  const mediumText = 'Useful context for this search result. '.repeat(4);
  await mockSnippets(page, [mediumText]);
  await page.setViewportSize({ width: 1100, height: 812 });
  await page.goto('/?q=needle');
  const row = page.locator('[data-result]');
  const snippet = row.locator('.result-snippet');
  const expand = row.getByRole('button', { name: 'Show more', exact: true });
  await expect(snippet).toHaveText(mediumText.trim());
  await page.evaluate(() => document.fonts.ready);
  await expectClipped(snippet, false);
  await expect(expand).toHaveCount(0);
  await page.setViewportSize({ width: 320, height: 812 });
  await expectClipped(snippet, true);
  await expect(expand).toBeVisible();
  await page.setViewportSize({ width: 1100, height: 812 });
  await expectClipped(snippet, false);
  await expect(expand).toHaveCount(0);

  await page.routeWebSocket('**/search', (socket) => {
    socket.onMessage((message) => {
      const request = JSON.parse(String(message));
      socket.send(
        JSON.stringify({
          documents: [
            {
              id: 'document-0',
              url: `${original}/0`,
              title: 'Article 1',
              text: request.text === 'short' ? 'Short replacement.' : longText,
            },
          ],
          total: 1,
        }),
      );
    });
  });
  await page.reload();
  await expectClipped(snippet, true);
  await expect(expand).toBeVisible();
  const search = page.getByRole('searchbox');
  await search.fill('short');
  await expect(snippet).toHaveText('Short replacement.');
  await expectClipped(snippet, false);
  await expect(expand).toHaveCount(0);
  await search.fill('needle');
  await expect(snippet).toContainText('End of the excerpt.');
  await expectClipped(snippet, true);
  await expect(expand).toBeVisible();
});
