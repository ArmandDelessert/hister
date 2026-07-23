type PageData = {
  title: string;
  text: string;
  url: string;
  html: string;
  faviconURL: string;
};

function getURL() {
  return window.location.href.replace(window.location.hash, '');
}

function extractPageData(): PageData {
  const d: PageData = {
    text: document.body?.innerText ?? '',
    title: document.querySelector('title')?.innerText ?? document.title,
    url: getURL(),
    html: document.documentElement?.innerHTML ?? '',
    faviconURL: new URL('/favicon.ico', getURL()).href,
  };
  const faviconHref = document.querySelector("link[rel~='icon']")?.getAttribute('href');
  if (faviconHref) {
    d.faviconURL = new URL(faviconHref, d.url).href;
  }
  return d;
}

export { type PageData, extractPageData };
