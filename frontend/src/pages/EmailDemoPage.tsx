import { useState } from 'react';
import './EmailDemoPage.css';

type Folder = 'inbox' | 'starred' | 'sent' | 'drafts' | 'spam' | 'trash';
type Email = { id: string; from: string; subject: string; preview: string; date: string; unread: boolean; starred: boolean; folder: Folder };

const EMAILS: Email[] = [
  { id: '1', from: 'María García', subject: 'Propuesta comercial Q2', preview: 'Te adjunto la propuesta actualizada con los nuevos precios…', date: 'Hoy 14:30', unread: true, starred: true, folder: 'inbox' },
  { id: '2', from: 'Juan Pérez', subject: 'Re: Reunión de equipo', preview: 'Confirmo asistencia para el jueves a las 10am…', date: 'Hoy 11:15', unread: true, starred: false, folder: 'inbox' },
  { id: '3', from: 'Stripe', subject: 'Pago recibido — INV-3492', preview: 'Se procesó correctamente el pago de $15,000…', date: 'Ayer', unread: false, starred: true, folder: 'inbox' },
  { id: '4', from: 'Ana López', subject: 'Feedback del cliente', preview: 'El cliente quedó muy contento con la demo…', date: 'Ayer', unread: false, starred: false, folder: 'inbox' },
  { id: '5', from: 'Carlos Ruiz', subject: 'Deploy a producción', preview: 'Todo listo para el deploy de mañana…', date: '23 Mar', unread: false, starred: false, folder: 'inbox' },
  { id: '6', from: 'Laura Díaz', subject: 'Nuevo diseño aprobado', preview: 'El cliente aprobó los mockups finales…', date: '22 Mar', unread: false, starred: true, folder: 'inbox' },
  { id: '7', from: 'Pedro Sánchez', subject: 'Cotización hosting', preview: 'Te paso la cotización anual del hosting…', date: '21 Mar', unread: false, starred: false, folder: 'sent' },
  { id: '8', from: 'Yo', subject: 'Notas reunión', preview: 'Borrador de las notas de la última reunión…', date: '20 Mar', unread: false, starred: false, folder: 'drafts' },
  { id: '9', from: 'Newsletter', subject: 'Ofertas de la semana', preview: 'No te pierdas las ofertas exclusivas…', date: '19 Mar', unread: true, starred: false, folder: 'spam' },
];

const FOLDERS: { id: Folder; label: string; icon: string }[] = [
  { id: 'inbox', label: 'Bandeja', icon: '📥' }, { id: 'starred', label: 'Destacados', icon: '⭐' },
  { id: 'sent', label: 'Enviados', icon: '📤' }, { id: 'drafts', label: 'Borradores', icon: '📝' },
  { id: 'spam', label: 'Spam', icon: '⚠️' }, { id: 'trash', label: 'Papelera', icon: '🗑️' },
];

export function EmailDemoPage() {
  const [folder, setFolder] = useState<Folder>('inbox');
  const [emails, setEmails] = useState(EMAILS);
  const [search, setSearch] = useState('');

  const toggleStar = (id: string) => setEmails(p => p.map(e => e.id === id ? { ...e, starred: !e.starred } : e));

  const filtered = emails.filter(e => {
    if (folder === 'starred') return e.starred;
    return e.folder === folder;
  }).filter(e => !search || e.from.toLowerCase().includes(search.toLowerCase()) || e.subject.toLowerCase().includes(search.toLowerCase()));

  const folderCounts = (f: Folder) => f === 'starred' ? emails.filter(e => e.starred).length : emails.filter(e => e.folder === f && e.unread).length;

  return (
    <div className="eml">
      <div className="page-header">
        <h1>Email</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>Bandeja de correo — demo inspirado en Wowdash</p>
      </div>
      <div className="eml__layout">
        <div className="eml__sidebar">
          {FOLDERS.map(f => {
            const count = folderCounts(f.id);
            return (
              <button key={f.id} type="button" className={`eml__folder ${folder === f.id ? 'eml__folder--active' : ''}`} onClick={() => setFolder(f.id)}>
                <span>{f.icon} {f.label}</span>
                {count > 0 && <span className="eml__folder-count">{count}</span>}
              </button>
            );
          })}
        </div>
        <div className="eml__main">
          <div className="eml__toolbar">
            <input type="search" className="eml__search" placeholder="Buscar emails…" value={search} onChange={e => setSearch(e.target.value)} />
          </div>
          <div className="eml__list">
            {filtered.length === 0 && <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-text-muted)' }}>Sin mensajes</div>}
            {filtered.map(e => (
              <div key={e.id} className={`eml__item ${e.unread ? 'eml__item--unread' : ''}`}>
                <span className={`eml__item-star ${e.starred ? 'eml__item-star--active' : ''}`} onClick={() => toggleStar(e.id)}>★</span>
                <span className="eml__item-from">{e.from}</span>
                <span className="eml__item-subject">{e.subject} — <span style={{ fontWeight: 400 }}>{e.preview}</span></span>
                <span className="eml__item-date">{e.date}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
export default EmailDemoPage;
