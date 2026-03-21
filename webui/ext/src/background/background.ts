import { sendPageData, sendResult } from '../modules/network';

const missingURLMsg = {
  error: 'Missing or invalid Hister server URL. Configure it in the addon popup.',
};

function setErrorBadge(tabId: number) {
  chrome.action.setBadgeText({ text: '!', tabId });
  chrome.action.setBadgeBackgroundColor({ color: '#ff4444', tabId });
}

function clearErrorBadge(tabId: number) {
  chrome.action.setBadgeText({ text: '', tabId });
}

// TODO check source
function cjsMsgHandler(request, sender, sendResponse) {
  chrome.storage.local
    .get(['histerURL', 'indexingEnabled', 'histerCustomHeaders'])
    .then((data) => {
      let u = data['histerURL'] || '';
      const indexingEnabled = data['indexingEnabled'] !== false;
      const customHeaders = Array.isArray(data['histerCustomHeaders'])
        ? data['histerCustomHeaders']
        : [];

      if (!u) {
        chrome.tabs.sendMessage(sender.tab.id, missingURLMsg);
        setErrorBadge(sender.tab.id);
        return;
      }
      if (!u.endsWith('/')) {
        u += '/';
      }
      if (request.pageData) {
        if (!indexingEnabled && request.action != 'reindex') {
          sendResponse({ status: 'disabled' });
          return;
        }
        sendPageData(u + 'api/add', request.pageData, customHeaders)
          .then((r) => {
            if (r.status === 201) {
              clearErrorBadge(sender.tab.id);
            } else if (r.status != 406) {
              setErrorBadge(sender.tab.id);
            }
            sendResponse({ status: 'ok', status_code: r.status });
          })
          .catch((err) => {
            setErrorBadge(sender.tab.id);
            sendResponse({ error: err.message });
          });
        return true;
      }
      if (request.resultData) {
        sendResult(u + 'api/history', request.resultData, customHeaders)
          .then((r) => {
            if (r.status === 201) {
              clearErrorBadge(sender.tab.id);
            } else if (r.status != 406) {
              setErrorBadge(sender.tab.id);
            }
            sendResponse({ status: 'ok', status_code: r.status });
          })
          .catch((err) => {
            setErrorBadge(sender.tab.id);
            sendResponse({ error: err.message });
          });
        return true;
      }
    })
    .catch((error) => {
      chrome.tabs.sendMessage(sender.tab.id, missingURLMsg);
      setErrorBadge(sender.tab.id);
    });
  return true;
}

chrome.runtime.onMessage.addListener(cjsMsgHandler);
