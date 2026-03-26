import { useState } from 'react';
import { IconStar } from '../components/Icons';
import './CryptoPage.css';

type CryptoTab = 'wallet' | 'marketplace' | 'portfolio';
type Coin = { id: string; name: string; symbol: string; icon: string; color: string; price: number; change: number; holding: number; allocation: number; supply: string; marketCap: string; spark: number[] };

const COINS: Coin[] = [
  { id: '1', name: 'Bitcoin', symbol: 'BTC', icon: '₿', color: '#f7931a', price: 67420, change: 2.4, holding: 0.15, allocation: 48, supply: '19.7M', marketCap: '$1.33T', spark: [40,55,48,65,60,72,68] },
  { id: '2', name: 'Ethereum', symbol: 'ETH', icon: 'Ξ', color: '#627eea', price: 3520, change: -1.2, holding: 2.5, allocation: 28, supply: '120M', marketCap: '$423B', spark: [50,45,55,42,48,52,46] },
  { id: '3', name: 'Solana', symbol: 'SOL', icon: '◎', color: '#9945ff', price: 142, change: 5.8, holding: 45, allocation: 12, supply: '441M', marketCap: '$63B', spark: [30,35,42,50,55,65,72] },
  { id: '4', name: 'Cardano', symbol: 'ADA', icon: '₳', color: '#0033ad', price: 0.62, change: -0.5, holding: 5000, allocation: 8, supply: '35.8B', marketCap: '$22B', spark: [55,50,48,45,42,44,40] },
  { id: '5', name: 'Polkadot', symbol: 'DOT', icon: '●', color: '#e6007a', price: 7.85, change: 3.1, holding: 200, allocation: 4, supply: '1.4B', marketCap: '$11B', spark: [35,40,38,45,50,55,58] },
];

function fmtUsd(n: number) { return n >= 1 ? `$${n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : `$${n.toFixed(4)}`; }

function Sparkline({ data, color }: { data: number[]; color: string }) {
  const max = Math.max(...data);
  return (
    <div className="cry__sparkline">
      {data.map((v, i) => <span key={i} style={{ height: `${(v / max) * 100}%`, background: color }} />)}
    </div>
  );
}

function WalletView({ coins }: { coins: Coin[] }) {
  const total = coins.reduce((s, c) => s + c.price * c.holding, 0);
  return (
    <>
      <div className="cry__stats">
        <div className="cry__stat-card"><div className="cry__stat-label">Valor total</div><div className="cry__stat-value">{fmtUsd(total)}</div></div>
        <div className="cry__stat-card"><div className="cry__stat-label">Activos</div><div className="cry__stat-value">{coins.length}</div></div>
        <div className="cry__stat-card"><div className="cry__stat-label">Mejor hoy</div><div className="cry__stat-value cry__change--up">+{Math.max(...coins.map(c => c.change)).toFixed(1)}%</div></div>
      </div>
      <div className="card">
        <table className="cry__table">
          <thead><tr><th>Activo</th><th>Cantidad</th><th>Precio</th><th>Valor</th><th>Cambio 24h</th><th>Asignación</th></tr></thead>
          <tbody>
            {coins.map(c => (
              <tr key={c.id}>
                <td><div className="cry__coin"><div className="cry__coin-icon" style={{ background: c.color }}>{c.icon}</div><div><strong>{c.name}</strong><br /><span style={{ fontSize: '.72rem', color: 'var(--color-text-muted)' }}>{c.symbol}</span></div></div></td>
                <td>{c.holding.toLocaleString()}</td>
                <td>{fmtUsd(c.price)}</td>
                <td style={{ fontWeight: 600 }}>{fmtUsd(c.price * c.holding)}</td>
                <td className={c.change >= 0 ? 'cry__change--up' : 'cry__change--down'}>{c.change >= 0 ? '+' : ''}{c.change}%</td>
                <td>{c.allocation}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function MarketView({ coins }: { coins: Coin[] }) {
  const [watched, setWatched] = useState<Set<string>>(new Set(['1', '3']));
  const toggleWatch = (id: string) => setWatched(p => { const n = new Set(p); n.has(id) ? n.delete(id) : n.add(id); return n; });
  return (
    <div className="card">
      <table className="cry__table">
        <thead><tr><th>Activo</th><th>Precio</th><th>Cambio 24h</th><th>Circ. Supply</th><th>Market Cap</th><th>24h</th><th>Watch</th></tr></thead>
        <tbody>
          {coins.map(c => (
            <tr key={c.id}>
              <td><div className="cry__coin"><div className="cry__coin-icon" style={{ background: c.color }}>{c.icon}</div><strong>{c.name}</strong></div></td>
              <td>{fmtUsd(c.price)}</td>
              <td className={c.change >= 0 ? 'cry__change--up' : 'cry__change--down'}>{c.change >= 0 ? '+' : ''}{c.change}%</td>
              <td>{c.supply}</td>
              <td>{c.marketCap}</td>
              <td><Sparkline data={c.spark} color={c.change >= 0 ? '#10b981' : '#ef4444'} /></td>
              <td><button type="button" onClick={() => toggleWatch(c.id)} style={{ border: 'none', background: 'transparent', cursor: 'pointer', fontSize: '1.1rem', color: watched.has(c.id) ? '#f59e0b' : 'var(--color-border)' }}><IconStar filled={watched.has(c.id)} /></button></td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function PortfolioView({ coins }: { coins: Coin[] }) {
  const total = coins.reduce((s, c) => s + c.price * c.holding, 0);
  return (
    <>
      <div className="cry__stats">
        <div className="cry__stat-card"><div className="cry__stat-label">Portfolio</div><div className="cry__stat-value">{fmtUsd(total)}</div></div>
        <div className="cry__stat-card"><div className="cry__stat-label">Ganancia 24h</div><div className="cry__stat-value cry__change--up">+$842</div></div>
      </div>
      <div className="card">
        <table className="cry__table">
          <thead><tr><th>Activo</th><th>Tu holding</th><th>Precio</th><th>Cambio</th><th>Asignación</th></tr></thead>
          <tbody>
            {coins.map(c => (
              <tr key={c.id}>
                <td><div className="cry__coin"><div className="cry__coin-icon" style={{ background: c.color }}>{c.icon}</div><strong>{c.symbol}</strong></div></td>
                <td>{c.holding.toLocaleString()} {c.symbol}</td>
                <td>{fmtUsd(c.price)}</td>
                <td className={c.change >= 0 ? 'cry__change--up' : 'cry__change--down'}>{c.change >= 0 ? '+' : ''}{c.change}%</td>
                <td>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '.4rem' }}>
                    <div style={{ flex: 1, height: 6, background: 'var(--color-border)', borderRadius: 3, overflow: 'hidden' }}>
                      <div style={{ height: '100%', width: `${c.allocation}%`, background: c.color, borderRadius: 3 }} />
                    </div>
                    <span style={{ fontSize: '.72rem', fontWeight: 600, color: 'var(--color-text-secondary)', minWidth: '2rem', textAlign: 'right' }}>{c.allocation}%</span>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

export function CryptoPage() {
  const [tab, setTab] = useState<CryptoTab>('wallet');
  return (
    <div className="cry">
      <div className="cry__tabs">
        {([['wallet', 'Billetera'], ['marketplace', 'Marketplace'], ['portfolio', 'Portfolio']] as const).map(([id, label]) => (
          <button key={id} type="button" className={`cry__tab ${tab === id ? 'cry__tab--active' : ''}`} onClick={() => setTab(id)}>{label}</button>
        ))}
      </div>
      {tab === 'wallet' && <WalletView coins={COINS} />}
      {tab === 'marketplace' && <MarketView coins={COINS} />}
      {tab === 'portfolio' && <PortfolioView coins={COINS} />}
    </div>
  );
}
export default CryptoPage;
