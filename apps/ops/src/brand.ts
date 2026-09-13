/**
 * Bal Setu brand constants (ops console copy).
 * Must stay in sync with apps/web/src/brand.ts.
 * A shared workspace package is future work once monorepo build is verified.
 */

export const BRAND = {
  name: 'Bal Setu',
  pageTitle: 'Bal Setu Staff Console',
  alertName: 'Nithari Alert',
  alertLabel: 'Nearby missing-child alerts',
  alertFallbackName: 'Setu Alert',
  subscriptionStorageKey: 'savera.subscription.v1',
  staffTitle: 'Bal Setu Staff',
  childHelpline: '1098',
  emergency: '112',
  exitUrl: 'https://news.google.com',
} as const;
