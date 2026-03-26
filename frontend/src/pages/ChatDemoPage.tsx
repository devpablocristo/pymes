import { useState, useRef, useEffect } from 'react';
import './ChatDemoPage.css';

type Contact = { id: string; name: string; initials: string; color: string; lastMsg: string };
type Msg = { id: string; contactId: string; text: string; fromMe: boolean; time: string };

const CONTACTS: Contact[] = [
  { id: '1', name: 'María García', initials: 'MG', color: '#3b82f6', lastMsg: 'Dale, hablamos mañana' },
  { id: '2', name: 'Juan Pérez', initials: 'JP', color: '#10b981', lastMsg: 'Perfecto, gracias!' },
  { id: '3', name: 'Ana López', initials: 'AL', color: '#8b5cf6', lastMsg: 'Te envío el presupuesto' },
  { id: '4', name: 'Carlos Ruiz', initials: 'CR', color: '#f59e0b', lastMsg: 'Listo el deploy' },
  { id: '5', name: 'Laura Díaz', initials: 'LD', color: '#ec4899', lastMsg: 'Quedó excelente!' },
];

const MESSAGES: Msg[] = [
  { id: '1', contactId: '1', text: 'Hola! Cómo va el proyecto?', fromMe: false, time: '14:20' },
  { id: '2', contactId: '1', text: 'Bien, estamos cerrando el sprint.', fromMe: true, time: '14:22' },
  { id: '3', contactId: '1', text: 'Genial! Necesitás algo de mi lado?', fromMe: false, time: '14:23' },
  { id: '4', contactId: '1', text: 'Sí, la aprobación del diseño para avanzar.', fromMe: true, time: '14:25' },
  { id: '5', contactId: '1', text: 'Dale, hablamos mañana', fromMe: false, time: '14:30' },
  { id: '6', contactId: '2', text: 'Juan, te mandé el acceso al repo.', fromMe: true, time: '10:00' },
  { id: '7', contactId: '2', text: 'Perfecto, gracias!', fromMe: false, time: '10:05' },
  { id: '8', contactId: '3', text: 'Ana, tenés el presupuesto listo?', fromMe: true, time: '09:00' },
  { id: '9', contactId: '3', text: 'Te envío el presupuesto', fromMe: false, time: '09:15' },
];

let nextMsgId = 100;

export function ChatDemoPage() {
  const [active, setActive] = useState('1');
  const [msgs, setMsgs] = useState(MESSAGES);
  const [input, setInput] = useState('');
  const [search, setSearch] = useState('');
  const endRef = useRef<HTMLDivElement>(null);

  const contact = CONTACTS.find(c => c.id === active)!;
  const thread = msgs.filter(m => m.contactId === active);

  useEffect(() => { endRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [thread.length, active]);

  const send = () => {
    if (!input.trim()) return;
    setMsgs(p => [...p, { id: String(++nextMsgId), contactId: active, text: input.trim(), fromMe: true, time: new Date().toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit' }) }]);
    setInput('');
  };

  const filteredContacts = CONTACTS.filter(c => !search || c.name.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="cht">
      <div className="page-header">
        <h1>Chat</h1>
        <p style={{ color: 'var(--color-text-secondary)', margin: 0, fontSize: '0.88rem' }}>Mensajería instantánea — demo</p>
      </div>
      <div className="cht__layout">
        <div className="cht__contacts">
          <div className="cht__contacts-header"><input className="cht__contacts-search" type="search" placeholder="Buscar…" value={search} onChange={e => setSearch(e.target.value)} /></div>
          <div className="cht__contacts-list">
            {filteredContacts.map(c => (
              <button key={c.id} type="button" className={`cht__contact ${active === c.id ? 'cht__contact--active' : ''}`} onClick={() => setActive(c.id)}>
                <div className="cht__contact-avatar" style={{ background: c.color }}>{c.initials}</div>
                <div className="cht__contact-info">
                  <div className="cht__contact-name">{c.name}</div>
                  <div className="cht__contact-preview">{c.lastMsg}</div>
                </div>
              </button>
            ))}
          </div>
        </div>
        <div className="cht__main">
          <div className="cht__header">{contact.name}</div>
          <div className="cht__messages">
            {thread.map(m => (
              <div key={m.id} className={`cht__msg ${m.fromMe ? 'cht__msg--me' : 'cht__msg--them'}`}>
                {m.text}
                <div className="cht__msg-time">{m.time}</div>
              </div>
            ))}
            <div ref={endRef} />
          </div>
          <div className="cht__input-bar">
            <input placeholder="Escribí un mensaje…" value={input} onChange={e => setInput(e.target.value)} onKeyDown={e => e.key === 'Enter' && send()} />
            <button type="button" className="btn-primary btn-sm" onClick={send}>Enviar</button>
          </div>
        </div>
      </div>
    </div>
  );
}
export default ChatDemoPage;
