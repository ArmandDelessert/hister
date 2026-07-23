// ==UserScript==
// @name         Hister for qutebrowser
// @namespace    https://github.com/asciimoo/hister
// @version      0.26.2
// @description  Automatically index rendered pages in Hister
// @match        http://*/*
// @match        https://*/*
// @run-at       document-idle
// @qute-js-world user
// @grant        none
// @noframes
// ==/UserScript==
// Edit these values after copying the script into qutebrowser.
const HISTER_QUTEBROWSER_CONFIG = Object.freeze({
	serverURL: "http://127.0.0.1:4433/",
	accessToken: "replace-with-app-access-token",
	label: ""
});
(function() {
	//#region src/modules/page-data.ts
	function getURL() {
		return window.location.href.replace(window.location.hash, "");
	}
	function extractPageData() {
		const d = {
			text: document.body?.innerText ?? "",
			title: document.querySelector("title")?.innerText ?? document.title,
			url: getURL(),
			html: document.documentElement?.innerHTML ?? "",
			faviconURL: new URL("/favicon.ico", getURL()).href
		};
		const faviconHref = document.querySelector("link[rel~='icon']")?.getAttribute("href");
		if (faviconHref) d.faviconURL = new URL(faviconHref, d.url).href;
		return d;
	}
	//#endregion
	//#region src/modules/network.ts
	var FETCHABLE_FAVICON_PROTOCOLS = /* @__PURE__ */ new Set(["http:", "https:"]);
	function isFetchableFaviconURL(rawURL) {
		try {
			const url = new URL(rawURL);
			return FETCHABLE_FAVICON_PROTOCOLS.has(url.protocol);
		} catch {
			return false;
		}
	}
	async function fetchFavicon(url) {
		if (!isFetchableFaviconURL(url)) return "";
		let iconBytes = await (await fetch(url)).blob();
		return new Promise((resolve) => {
			const reader = new FileReader();
			reader.onloadend = () => {
				resolve(typeof reader.result === "string" ? reader.result : "");
			};
			reader.onerror = () => resolve("");
			reader.readAsDataURL(iconBytes);
		});
	}
	//#endregion
	//#region src/qutebrowser/qutebrowser.ts
	var defaultSleepTime = 10 * 1e3;
	var maximumSleepTime = 600 * 1e3;
	var pageData = null;
	var sleepTime = defaultSleepTime;
	var updateTimer = null;
	function serverBaseURL() {
		const baseURL = new URL(HISTER_QUTEBROWSER_CONFIG.serverURL);
		if (baseURL.protocol !== "http:" && baseURL.protocol !== "https:") throw new Error("the Hister server URL must use HTTP or HTTPS");
		if (!baseURL.pathname.endsWith("/")) baseURL.pathname += "/";
		baseURL.search = "";
		baseURL.hash = "";
		return baseURL;
	}
	function addEndpoint() {
		return new URL("api/add", serverBaseURL()).href;
	}
	function isHisterPage() {
		const currentURL = new URL(window.location.href);
		const baseURL = serverBaseURL();
		const basePathWithoutSlash = baseURL.pathname.slice(0, -1);
		return currentURL.origin === baseURL.origin && (currentURL.pathname === basePathWithoutSlash || currentURL.pathname.startsWith(baseURL.pathname));
	}
	function configurationError() {
		if (!HISTER_QUTEBROWSER_CONFIG.serverURL) return "set serverURL in the Hister qutebrowser userscript";
		if (!HISTER_QUTEBROWSER_CONFIG.accessToken || HISTER_QUTEBROWSER_CONFIG.accessToken === "replace-with-app-access-token") return "set accessToken in the Hister qutebrowser userscript";
		try {
			serverBaseURL();
		} catch (error) {
			return error instanceof Error ? error.message : "set a valid Hister server URL";
		}
		return null;
	}
	async function submit(data) {
		const fields = /* @__PURE__ */ new Map([
			["url", data.url],
			["title", data.title],
			["text", data.text],
			["html", data.html],
			["access_token", HISTER_QUTEBROWSER_CONFIG.accessToken],
			["hister_client", "greasemonkey"]
		]);
		if (HISTER_QUTEBROWSER_CONFIG.label) fields.set("label", HISTER_QUTEBROWSER_CONFIG.label);
		try {
			const favicon = await fetchFavicon(data.faviconURL);
			if (favicon) fields.set("favicon", favicon);
		} catch (error) {
			console.debug("Hister could not read the page favicon", error);
		}
		const host = document.createElement("div");
		host.hidden = true;
		const shadowRoot = host.attachShadow({ mode: "closed" });
		const form = document.createElement("form");
		form.action = addEndpoint();
		form.method = "post";
		form.enctype = "application/x-www-form-urlencoded";
		for (const [name, value] of fields) {
			const input = document.createElement("input");
			input.type = "hidden";
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
	function scheduleUpdate() {
		if (updateTimer !== null) clearTimeout(updateTimer);
		updateTimer = setTimeout(update, sleepTime);
	}
	function readPageData() {
		try {
			return extractPageData();
		} catch (error) {
			console.error("Hister failed to extract page data", error);
			return null;
		}
	}
	function pageChanged(nextPageData) {
		return pageData === null || nextPageData.html !== pageData.html || nextPageData.url !== pageData.url;
	}
	async function submitWithLogging(data) {
		try {
			await submit(data);
		} catch (error) {
			console.error("Hister failed to submit page data", error);
		}
	}
	async function update() {
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
	function start() {
		const error = configurationError();
		if (error !== null) {
			console.error(`Hister qutebrowser userscript is not configured: ${error}`);
			return;
		}
		if (isHisterPage()) return;
		if (typeof window.navigation !== "undefined") window.navigation.addEventListener("navigatesuccess", () => void update());
		update();
	}
	if (document.readyState === "complete") start();
	else window.addEventListener("load", start, { once: true });
	//#endregion
})();
