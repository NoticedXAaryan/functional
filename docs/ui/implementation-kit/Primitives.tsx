/* Copy into a shared UI package only after resolving the monorepo import strategy.
   These are presentation primitives, not complete application flows. No fake APIs. */
import { useId, type ReactNode } from 'react';

type ButtonProps = {
  children: ReactNode;
  onClick?: () => void;
  type?: 'button' | 'submit';
  disabled?: boolean;
  disabledReason?: string;
  variant?: 'primary' | 'secondary' | 'danger';
};
export function ActionButton({ children, onClick, type = 'button', disabled = false,
  disabledReason, variant = 'primary' }: ButtonProps) {
  const reasonId = useId();
  return <div><button type={type} onClick={onClick} disabled={disabled}
    aria-describedby={disabled && disabledReason ? reasonId : undefined}
    className={`bs-button${variant === 'primary' ? '' : ` bs-button--${variant}`}`}>
    {children}</button>{disabled && disabledReason && <p className="bs-help" id={reasonId}>{disabledReason}</p>}</div>;
}

export function ChoiceButton({ label, description, icon, selected, onChoose,
  tone = 'lilac' }: {label: string; description?: string; icon: string;
    selected?: boolean; onChoose: () => void; tone?: 'lilac' | 'sky' | 'peach'}) {
  return <button type="button" className={`bs-choice bs-choice--${tone}`}
    aria-pressed={selected} onClick={onChoose}>
    <img src={icon} width={32} height={32} alt="" />
    <span><strong>{label}</strong>{description && <span style={{ display: 'block' }}>{description}</span>}</span>
    {selected && <span aria-hidden="true">✓</span>}
  </button>;
}

export function ChildShell({ children, brand, languageControl, leaveLabel, onLeave,
  footer }: {children: ReactNode; brand: string; languageControl: ReactNode;
    leaveLabel: string; onLeave: () => void; footer: ReactNode}) {
  return <div className="bs-ui"><header className="bs-header">
    <a className="bs-brand" href="/">{brand}</a>{languageControl}
    <button type="button" className="bs-button bs-button--secondary" onClick={onLeave}>{leaveLabel}</button>
  </header><main className="bs-main" id="main-content">{children}</main>
  <footer className="bs-footer">{footer}</footer></div>;
}

export function InlineError({ message, retryLabel, onRetry }: {
  message: string; retryLabel: string; onRetry?: () => void;
}) {
  return <div className="bs-error"><p role="alert">{message}</p>
    {onRetry && <button type="button" className="bs-button bs-button--secondary" onClick={onRetry}>{retryLabel}</button>}
  </div>;
}

// Private-media authorization/refresh must be implemented by the host application.
// Native audio is intentionally used: keyboard support and no auto-play.
export function VoicePlayer({ source, title, soundWarning, equivalentText, onError }: {
  source: string; title: string; soundWarning: string; equivalentText?: string; onError: () => void;
}) {
  const id = useId();
  return <section className="bs-card" aria-labelledby={id}>
    <h2 id={id}>{title}</h2><p className="bs-help">{soundWarning}</p>
    <audio controls preload="none" src={source} onError={onError}
      aria-label={title} style={{ width: '100%' }} />
    {equivalentText && <p>{equivalentText}</p>}
  </section>;
}
