/**
 * Bal Setu brand constants.
 * Single source of truth for visible names, alert labels and compatibility aliases.
 * Do not duplicate these strings across apps or worker templates.
 *
 * Compatibility rules (from DECISIONS.md and doc/01):
 * - savera.subscription.v1 localStorage key is retained until an explicit credential-preserving migration exists
 * - /alerts/{id} routes are unchanged
 * - /savera.svg alias must continue to resolve for installed PWAs and old service workers
 */

export const BRAND = {
  /** Platform name shown in UI, HTML title and manifest */
  name: 'Bal Setu',

  /** Full platform title for HTML <title> elements */
  pageTitle: 'Bal Setu — Private help for children',

  /** Alert feature name. Subject to operator/community review; fall back to ALERT_FALLBACK_NAME if declined. */
  alertName: 'Nithari Alert',

  /** Descriptive label shown on every alert entry point */
  alertLabel: 'Nearby missing-child alerts',

  /** Fallback alert name if the incident name is declined by affected community review */
  alertFallbackName: 'Setu Alert',

  /** Key for notification subscription management — DO NOT RENAME */
  subscriptionStorageKey: 'savera.subscription.v1',

  /** Staff console name */
  staffTitle: 'Bal Setu Staff',

  /** Immediate-help phone */
  childHelpline: '1098',

  /** Emergency phone */
  emergency: '112',

  /** Leave-page destination (neutral HTTPS) */
  exitUrl: 'https://news.google.com',
} as const;

export type BrandKeys = keyof typeof BRAND;
