// Theme is a data-attribute on <html>, persisted so it survives reloads.
const THEME_KEY = 'mailfold.theme';

export function getTheme() {
  return (
    localStorage.getItem(THEME_KEY) ||
    document.documentElement.getAttribute('data-theme') ||
    'light'
  );
}

export function applyTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  localStorage.setItem(THEME_KEY, theme);
}
