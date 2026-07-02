import React, { createContext, useContext, useState, useCallback, useMemo, useEffect } from 'react';
import en from './locales/en.json';

// Registry of UI languages. Only `enabled` locales appear in the picker. Shipping
// a new language is a two-step drop-in: add src/i18n/locales/<code>.json, register
// it in RESOURCES below, and flip enabled:true here — no other code changes.
export const LOCALES = [
  { code: 'en', name: 'English', nativeName: 'English', dir: 'ltr', enabled: true },
  // Scaffolded for the future — translations not provided yet:
  // { code: 'ru', name: 'Russian', nativeName: 'Русский', dir: 'ltr', enabled: false },
  // { code: 'de', name: 'German',  nativeName: 'Deutsch', dir: 'ltr', enabled: false },
];

const RESOURCES = { en };
const DEFAULT = 'en';
const STORAGE = 'mailfold.lang';

function resolve(obj, key) {
  return key.split('.').reduce((o, k) => (o == null ? undefined : o[k]), obj);
}

function interpolate(str, vars) {
  if (!vars || typeof str !== 'string') return str;
  return str.replace(/\{\{\s*(\w+)\s*\}\}/g, (m, k) => (vars[k] != null ? String(vars[k]) : m));
}

// translate looks a key up in the active locale, falling back to English and then
// to the key itself. When a `count` var is present it selects a plural form using
// the i18next-style `key_<category>` suffix convention (e.g. count_one/count_other).
function translate(lang, key, vars) {
  const primary = RESOURCES[lang] || RESOURCES[DEFAULT];
  const fallback = RESOURCES[DEFAULT];

  const lookup = k => {
    let v = resolve(primary, k);
    if (v === undefined && primary !== fallback) v = resolve(fallback, k);
    return v;
  };

  if (vars && typeof vars.count === 'number') {
    const category = new Intl.PluralRules(lang).select(vars.count);
    const plural = lookup(`${key}_${category}`);
    if (plural !== undefined) return interpolate(plural, vars);
    const other = lookup(`${key}_other`);
    if (other !== undefined) return interpolate(other, vars);
  }

  const val = lookup(key);
  if (val === undefined || typeof val === 'object') return key;
  return interpolate(val, vars);
}

const I18nCtx = createContext(null);

export function I18nProvider({ children }) {
  const [lang, setLangState] = useState(() => localStorage.getItem(STORAGE) || DEFAULT);

  useEffect(() => {
    document.documentElement.lang = lang;
    const meta = LOCALES.find(l => l.code === lang);
    document.documentElement.dir = (meta && meta.dir) || 'ltr';
  }, [lang]);

  const setLang = useCallback(code => {
    if (!RESOURCES[code]) return;
    localStorage.setItem(STORAGE, code);
    setLangState(code);
  }, []);

  const t = useCallback((key, vars) => translate(lang, key, vars), [lang]);

  const value = useMemo(
    () => ({ lang, setLang, t, locales: LOCALES.filter(l => l.enabled) }),
    [lang, setLang, t],
  );

  return <I18nCtx.Provider value={value}>{children}</I18nCtx.Provider>;
}

export function useI18n() {
  return useContext(I18nCtx);
}

// useT is the common case — just the translate function.
export function useT() {
  return useContext(I18nCtx).t;
}
