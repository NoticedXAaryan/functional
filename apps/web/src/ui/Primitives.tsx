/**
 * Bal Setu presentation primitives.
 * Adapted from docs/ui/implementation-kit/Primitives.tsx with accessibility requirements
 * from COMPONENTS.md applied. No fake APIs, auth, recording, or navigation history.
 *
 * Import tokens once: import '../bs-tokens.css';
 * Wrap migrated screens with className="bs-ui".
 */
import { useId, type ReactNode } from 'react';

// ── ActionButton ────────────────────────────────────────────────────────────
// type="submit" only inside an owning form.
// Pending state: parent must supply an accessible status announcement.
// disabledReason is shown below the button so it is still announced.
type ButtonVariant = 'primary' | 'secondary' | 'danger';

type ActionButtonProps = {
  children: ReactNode;
  onClick?: () => void;
  type?: 'button' | 'submit';
  disabled?: boolean;
  disabledReason?: string;
  variant?: ButtonVariant;
  id?: string;
};

export function ActionButton({
  children,
  onClick,
  type = 'button',
  disabled = false,
  disabledReason,
  variant = 'primary',
  id,
}: ActionButtonProps) {
  const reasonId = useId();
  return (
    <div>
      <button
        id={id}
        type={type}
        onClick={onClick}
        disabled={disabled}
        aria-describedby={disabled && disabledReason ? reasonId : undefined}
        className={`bs-button${variant === 'primary' ? '' : ` bs-button--${variant}`}`}
      >
        {children}
      </button>
      {disabled && disabledReason && (
        <p className="bs-help" id={reasonId}>
          {disabledReason}
        </p>
      )}
    </div>
  );
}

// ── ChoiceButton ────────────────────────────────────────────────────────────
// onChoose updates one controlled value. No send side effect.
// For single selection, the parent must implement labelled radio-group semantics
// (or use a native radio group). aria-pressed alone does not form a radio group.
type ChoiceTone = 'lilac' | 'sky' | 'peach';

type ChoiceButtonProps = {
  label: string;
  description?: string;
  iconSrc: string;
  selected?: boolean;
  onChoose: () => void;
  tone?: ChoiceTone;
  id?: string;
};

export function ChoiceButton({
  label,
  description,
  iconSrc,
  selected = false,
  onChoose,
  tone = 'lilac',
  id,
}: ChoiceButtonProps) {
  return (
    <button
      id={id}
      type="button"
      className={`bs-choice bs-choice--${tone}`}
      aria-pressed={selected}
      onClick={onChoose}
    >
      <img src={iconSrc} width={32} height={32} alt="" />
      <span>
        <strong>{label}</strong>
        {description && <span style={{ display: 'block', fontWeight: 400, fontSize: '0.9em' }}>{description}</span>}
      </span>
      {selected && <span aria-hidden="true">✓</span>}
    </button>
  );
}

// ── ChildShell ──────────────────────────────────────────────────────────────
// onLeave must run the complete exit sequence from COMPONENTS.md.
// languageControl and footer are required, not optional placeholders.
// Add skip link and heading focus management when integrating routing.
type ChildShellProps = {
  children: ReactNode;
  /** Brand mark element — app icon img + text "Bal Setu" */
  brand: ReactNode;
  /** Language picker element */
  languageControl: ReactNode;
  leaveLabel: string;
  onLeave: () => void;
  footer: ReactNode;
};

export function ChildShell({
  children,
  brand,
  languageControl,
  leaveLabel,
  onLeave,
  footer,
}: ChildShellProps) {
  return (
    <div className="bs-ui">
      <a className="bs-sr-only" href="#main-content">
        Skip to main content
      </a>
      <header className="bs-header">
        <div className="bs-brand">{brand}</div>
        {languageControl}
        <button
          type="button"
          className="bs-button bs-button--secondary"
          onClick={onLeave}
          aria-label={`${leaveLabel} — changes this page`}
        >
          {leaveLabel}
        </button>
      </header>
      <main className="bs-main" id="main-content" tabIndex={-1}>
        {children}
      </main>
      <footer className="bs-footer">{footer}</footer>
    </div>
  );
}

// ── InlineError ─────────────────────────────────────────────────────────────
// Translate stable server error codes; never pass raw server text with PII or technical data.
// onRetry reuses the same previous operation.
type InlineErrorProps = {
  message: string;
  retryLabel: string;
  onRetry?: () => void;
};

export function InlineError({ message, retryLabel, onRetry }: InlineErrorProps) {
  return (
    <div className="bs-error">
      <p role="alert">{message}</p>
      {onRetry && (
        <button type="button" className="bs-button bs-button--secondary" onClick={onRetry}>
          {retryLabel}
        </button>
      )}
    </div>
  );
}

// ── VoicePlayer ─────────────────────────────────────────────────────────────
// Caller must pass an authorized same-origin blob URL.
// Fetch with bearer auth → create short-lived Blob URL → pass as source → revoke on teardown.
// Never put a bearer token in the audio URL. No autoplay.
// Caller must stop every player on exit and permission changes.
type VoicePlayerProps = {
  /** Short-lived authorized blob URL from caller-controlled fetch */
  source: string;
  title: string;
  soundWarning: string;
  equivalentText?: string;
  onError: () => void;
};

export function VoicePlayer({
  source,
  title,
  soundWarning,
  equivalentText,
  onError,
}: VoicePlayerProps) {
  const id = useId();
  return (
    <section className="bs-card" aria-labelledby={id}>
      <h2 id={id}>{title}</h2>
      <p className="bs-help">{soundWarning}</p>
      <audio
        controls
        preload="none"
        src={source}
        onError={onError}
        aria-label={title}
        style={{ width: '100%' }}
      />
      {equivalentText && <p>{equivalentText}</p>}
    </section>
  );
}
