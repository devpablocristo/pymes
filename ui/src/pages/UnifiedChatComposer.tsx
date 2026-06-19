import { PendingConfirmationsPanel } from '@devpablocristo/platform-chat-ui';
import '@devpablocristo/platform-chat-ui/styles.css';

type ChatComposerProps = {
  input: string;
  busy: boolean;
  inputPrompt: string;
  pendingConfirmations: string[];
  onInputChange: (value: string) => void;
  onSend: () => void;
  onConfirmPending: () => void;
  t: (key: string, variables?: Record<string, string | number>) => string;
};

export function ChatComposer({
  input,
  busy,
  inputPrompt,
  pendingConfirmations,
  onInputChange,
  onSend,
  onConfirmPending,
  t,
}: ChatComposerProps) {
  return (
    <>
      <PendingConfirmationsPanel
        items={pendingConfirmations}
        busy={busy}
        title="Pendientes"
        confirmLabel="Confirmar acciones"
        onConfirm={onConfirmPending}
      />
      <div className="cht__input-bar">
        <input
          aria-label={inputPrompt}
          placeholder={inputPrompt}
          value={input}
          disabled={busy}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              onSend();
            }
          }}
        />
        <button type="button" className="btn-primary btn-sm" disabled={busy || !input.trim()} onClick={onSend}>
          {busy ? t('ai.chat.sending') : t('ai.chat.send')}
        </button>
      </div>
    </>
  );
}
