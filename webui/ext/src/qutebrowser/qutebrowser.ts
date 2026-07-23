import { type PageData, extractPageData } from '../modules/page-data';
import { fetchFavicon } from '../modules/network';

declare const HISTER_QUTEBROWSER_CONFIG: {
  serverURL: string;
  accessToken: string;
  label: string;
};

const defaultSleepTime = 10 * 1000;
const maximumSleepTime = 10 * 60 * 1000;
let pageData: PageData | null = null;
let sleepTime = defaultSleepTime;
let updateTimer: ReturnType<typeof setTimeout> | null = null;

function serverBaseURL(): URL {
  const baseURL = new URL(HISTER_QUTEBROWSER_CONFIG.serverURL);
  if (baseURL.protocol !== 'http:' && baseURL.protocol !== 'https:') {
    throw new Error('the Hister server URL must use HTTP or HTTPS');
  }
  if (!baseURL.pathname.endsWith('/')) {
    baseURL.pathname += '/';
  }
  baseURL.search = '';
  baseURL.hash = '';
  return baseURL;
}

function addEndpoint(): string {
  return new URL('api/add', serverBaseURL()).href;
}

function isHisterPage(): boolean {
  const currentURL = new URL(window.location.href);
  const baseURL = serverBaseURL();
  const basePathWithoutSlash = baseURL.pathname.slice(0, -1);
  return (
    currentURL.origin === baseURL.origin &&
    (currentURL.pathname === basePathWithoutSlash ||
      currentURL.pathname.startsWith(baseURL.pathname))
  );
}

function configurationError(): string | null {
  if (!HISTER_QUTEBROWSER_CONFIG.serverURL) {
    return 'set serverURL in the Hister qutebrowser userscript';
  }
  if (
    !HISTER_QUTEBROWSER_CONFIG.accessToken ||
    HISTER_QUTEBROWSER_CONFIG.accessToken === 'replace-with-app-access-token'
  ) {
    return 'set accessToken in the Hister qutebrowser userscript';
  }
  try {
    serverBaseURL();
  } catch (error) {
    return error instanceof Error ? error.message : 'set a valid Hister server URL';
  }
  return null;
}

async function submit(data: PageData): Promise<void> {
  const fields = new Map<string, string>([
    ['url', data.url],
    ['title', data.title],
    ['text', data.text],
    ['html', data.html],
    ['access_token', HISTER_QUTEBROWSER_CONFIG.accessToken],
    ['hister_client', 'greasemonkey'],
  ]);
  if (HISTER_QUTEBROWSER_CONFIG.label) {
    fields.set('label', HISTER_QUTEBROWSER_CONFIG.label);
  }
  try {
    const favicon = await fetchFavicon(data.faviconURL);
    if (favicon) {
      fields.set('favicon', favicon);
    }
  } catch (error) {
    console.debug('Hister could not read the page favicon', error);
  }

  const host = document.createElement('div');
  host.hidden = true;
  const shadowRoot = host.attachShadow({ mode: 'closed' });
  const form = document.createElement('form');
  form.action = addEndpoint();
  form.method = 'post';
  form.enctype = 'application/x-www-form-urlencoded';
  for (const [name, value] of fields) {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    input.value = value;
    form.append(input);
  }
  shadowRoot.append(form);
  document.documentElement.append(host);
  try {
    form.submit();
  } finally {
    host.remove();
  }
}

function scheduleUpdate(): void {
  if (updateTimer !== null) {
    clearTimeout(updateTimer);
  }
  updateTimer = setTimeout(update, sleepTime);
}

function readPageData(): PageData | null {
  try {
    return extractPageData();
  } catch (error) {
    console.error('Hister failed to extract page data', error);
    return null;
  }
}

function pageChanged(nextPageData: PageData): boolean {
  return (
    pageData === null || nextPageData.html !== pageData.html || nextPageData.url !== pageData.url
  );
}

async function submitWithLogging(data: PageData): Promise<void> {
  try {
    await submit(data);
  } catch (error) {
    console.error('Hister failed to submit page data', error);
  }
}

async function update(): Promise<void> {
  const nextPageData = readPageData();
  if (nextPageData === null) {
    scheduleUpdate();
    return;
  }
  if (!pageChanged(nextPageData)) {
    sleepTime = Math.min(sleepTime * 2, maximumSleepTime);
    scheduleUpdate();
    return;
  }

  pageData = nextPageData;
  sleepTime = defaultSleepTime;
  await submitWithLogging(nextPageData);
  scheduleUpdate();
}

function start(): void {
  const error = configurationError();
  if (error !== null) {
    console.error(`Hister qutebrowser userscript is not configured: ${error}`);
    return;
  }
  if (isHisterPage()) {
    return;
  }
  if (typeof window.navigation !== 'undefined') {
    window.navigation.addEventListener('navigatesuccess', () => void update());
  }
  void update();
}

if (document.readyState === 'complete') {
  start();
} else {
  window.addEventListener('load', start, { once: true });
}
