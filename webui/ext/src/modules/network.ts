async function fetchFavicon(url) {
  const response = await fetch(url);
  let iconBytes = await response.blob();
  const reader = new FileReader();
  reader.readAsDataURL(iconBytes);
  //let icon = btoa(iconBytes.text());
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onloadend = () => {
      resolve(reader.result);
    };
    reader.onerror = () => resolve('');
    reader.readAsDataURL(iconBytes);
  });
}

async function getServerCookies(): Promise<string> {
  return new Promise((resolve) => {
    chrome.storage.local.get(['histerCookies'], (data) => {
      resolve(data['histerCookies'] || '');
    });
  });
}

async function fetchAPI(
  url: string,
  options: {
    method?: string;
    body?: unknown;
    customHeaders?: { name: string; value: string }[];
  } = {},
): Promise<Response> {
  const cookieHeader = await getServerCookies();
  const headers: Record<string, string> = {};

  if (options.body !== undefined) {
    headers['Content-type'] = 'application/json; charset=UTF-8';
  }
  if (cookieHeader) {
    headers['Cookie'] = cookieHeader;
  }
  for (const h of options.customHeaders ?? []) {
    if (h.name) headers[h.name] = h.value || '';
  }

  const fetchOptions: RequestInit = {
    method: options.method ?? (options.body !== undefined ? 'POST' : 'GET'),
    headers,
  };
  if (options.body !== undefined) {
    fetchOptions.body = JSON.stringify(options.body);
  }

  return fetch(url, fetchOptions);
}

async function sendPageData(url, doc, customHeaders = []) {
  try {
    doc['favicon'] = await fetchFavicon(doc.faviconURL);
  } catch (e) {
    doc['favicon'] = '';
  }
  return sendResult(url, doc, customHeaders);
}

async function sendResult(url, res, customHeaders = []) {
  return fetchAPI(url, { body: res, customHeaders });
}

export { fetchAPI, sendPageData, sendResult };
