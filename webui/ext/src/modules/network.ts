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

async function sendPageData(url, doc, customHeaders = []) {
  try {
    doc['favicon'] = await fetchFavicon(doc.faviconURL);
  } catch (e) {
    doc['favicon'] = '';
  }
  return sendResult(url, doc, customHeaders);
}

async function sendResult(url, res, customHeaders = []) {
  const cookieHeader = await getServerCookies();

  const headers: Record<string, string> = {
    'Content-type': 'application/json; charset=UTF-8',
  };
  if (cookieHeader) {
    headers['Cookie'] = cookieHeader;
  }
  for (const h of customHeaders) {
    if (h.name) {
      headers[h.name] = h.value || '';
    }
  }
  return fetch(url, {
    method: 'POST',
    body: JSON.stringify(res),
    headers,
  });
}

export { sendPageData, sendResult };
