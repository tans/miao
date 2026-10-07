// UI language runtime. zh-CN strings are the source of truth used directly in
// templates and code; the en dictionary maps them to translations. Adding a
// language is a pure dictionary extension (messages_<lang>) plus one entry in
// LANGS and LOCALES.
export const LANGS = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en', label: 'English' }
];

const LOCALES = { 'zh-CN': 'zh-CN', en: 'en' };

const messages = {
  en: {}
};

let accountLanguage = '';
let currentLanguage = resolve();
const listeners = new Set();
let persistHandler = null;

function resolve() {
  const explicit = normalize(new URLSearchParams(location.search).get('lang'));
  if (explicit) return explicit;
  const saved = normalize(localStorage.getItem('miao_lang'));
  if (saved) return saved;
  for (const candidate of navigator.languages || []) {
    const lang = normalize(candidate);
    if (lang) return lang;
  }
  return 'zh-CN';
}

function normalize(raw) {
  const value = String(raw || '').trim().toLowerCase();
  if (value === 'en' || value.startsWith('en-')) return 'en';
  if (value === 'zh' || value.startsWith('zh-cn') || value.startsWith('zh-sg') || value.startsWith('zh-hans')) return 'zh-CN';
  return '';
}

function effective() {
  const explicit = normalize(new URLSearchParams(location.search).get('lang'));
  if (explicit) return explicit;
  if (accountLanguage) return accountLanguage;
  const saved = normalize(localStorage.getItem('miao_lang'));
  if (saved) return saved;
  for (const candidate of navigator.languages || []) {
    const lang = normalize(candidate);
    if (lang) return lang;
  }
  return 'zh-CN';
}

export function getLanguage() {
  return currentLanguage;
}

export function languageLabel(code) {
  return LANGS.find((item) => item.code === code)?.label || code;
}

// t translates a source zh-CN string with optional {name} interpolation.
export function t(key, params) {
  let message = key;
  if (currentLanguage !== 'zh-CN') message = messages[currentLanguage]?.[key] ?? key;
  if (!params) return message;
  return message.replace(/\{(\w+)\}/g, (match, name) => (name in params ? String(params[name]) : match));
}

// applyTranslations renders data-i18n attributes after the language changes.
// The zh-CN source text lives in the document itself, so an empty attribute
// uses the current text/placeholder as the dictionary key.
export function applyTranslations(root = document) {
  for (const node of root.querySelectorAll('[data-i18n]')) {
    const key = node.dataset.i18n || node.textContent.trim();
    node.dataset.i18n = key;
    node.textContent = t(key);
  }
  for (const node of root.querySelectorAll('[data-i18n-placeholder]')) {
    const key = node.dataset.i18nPlaceholder || node.getAttribute('placeholder') || '';
    node.dataset.i18nPlaceholder = key;
    node.setAttribute('placeholder', t(key));
  }
  for (const node of root.querySelectorAll('[data-i18n-aria-label]')) {
    const key = node.dataset.i18nAriaLabel || node.getAttribute('aria-label') || '';
    node.dataset.i18nAriaLabel = key;
    node.setAttribute('aria-label', t(key));
  }
  for (const node of root.querySelectorAll('[data-i18n-title]')) {
    const key = node.dataset.i18nTitle || node.getAttribute('title') || '';
    node.dataset.i18nTitle = key;
    node.setAttribute('title', t(key));
  }
}

export function onChange(listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

// setPersistHandler routes account persistence (PATCH /api/me) to app code;
// i18n stays independent of the API layer.
export function setPersistHandler(handler) {
  persistHandler = handler;
}

// setAccountLanguage applies the logged-in account's saved language without
// re-login; empty follows the browser.
export function setAccountLanguage(code) {
  accountLanguage = normalize(code);
  apply(effective(), { persist: false });
}

export async function setLanguage(code, { persist = true } = {}) {
  const lang = normalize(code);
  if (!lang) return;
  if (persist) localStorage.setItem('miao_lang', lang);
  if (persist && persistHandler) await persistHandler(lang).catch(() => {});
  apply(lang, { persist });
}

function apply(lang) {
  if (lang === currentLanguage) return;
  currentLanguage = lang;
  document.documentElement.lang = LOCALES[lang] || lang;
  applyTranslations();
  for (const listener of listeners) listener(lang);
}

// Explicit ?lang= choices also stick for anonymous visitors.
const urlLang = normalize(new URLSearchParams(location.search).get('lang'));
if (urlLang && !localStorage.getItem('miao_lang')) localStorage.setItem('miao_lang', urlLang);

document.documentElement.lang = LOCALES[currentLanguage] || currentLanguage;

const formatter = (options) => (value) => {
  if (value == null || value === '') return '';
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat(LOCALES[currentLanguage] || currentLanguage, options).format(date);
};

export const fmtDate = formatter({ year: 'numeric', month: 'short', day: 'numeric' });
export const fmtDateTime = formatter({ year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
export const fmtTime = formatter({ hour: '2-digit', minute: '2-digit' });

export function fmtNumber(value) {
  if (value == null || value === '') return '';
  const numeric = Number(value);
  if (Number.isNaN(numeric)) return String(value);
  return new Intl.NumberFormat(LOCALES[currentLanguage] || currentLanguage).format(numeric);
}
