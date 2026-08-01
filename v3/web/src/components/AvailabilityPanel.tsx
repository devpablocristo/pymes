import { DateTime } from "luxon";
import type { AvailabilityBlock, AvailabilityRule, Resource } from "../domain/scheduling";

const weekday = ["domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"];

function clock(minute: number): string {
  const hours = Math.floor(minute / 60) % 24;
  const minutes = minute % 60;
  const suffix = minute >= 1440 ? " +1" : "";
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}${suffix}`;
}

export function AvailabilityPanel({
  rules,
  blocks,
  resources,
  timezone,
  canManage,
  onCreateBlock,
}: {
  rules: AvailabilityRule[];
  blocks: AvailabilityBlock[];
  resources: Resource[];
  timezone: string;
  canManage: boolean;
  onCreateBlock: () => void;
}) {
  return (
    <div className="operation-panel">
      <section className="operation-panel__main">
        <header className="section-heading">
          <div>
            <p className="eyebrow">Reglas locales · {timezone}</p>
            <h2>Disponibilidad habitual</h2>
          </div>
        </header>
        {rules.length ? (
          <div className="rule-list">
            {rules.map((rule) => (
              <article key={rule.id}>
                <div className="rule-list__day">{weekday[rule.weekday]}</div>
                <strong>
                  {clock(rule.start_minute)}–{clock(rule.end_minute)}
                </strong>
                <span>{rule.resource_id ? resources.find((item) => item.id === rule.resource_id)?.name : "Sucursal"}</span>
              </article>
            ))}
          </div>
        ) : (
          <div className="empty-state">
            <h3>No hay horarios habituales</h3>
            <p>Configurá reglas semanales antes de abrir el booking público.</p>
          </div>
        )}
      </section>
      <section className="operation-panel__side">
        <header className="section-heading">
          <div>
            <p className="eyebrow">Excepciones</p>
            <h2>Bloqueos próximos</h2>
          </div>
          {canManage ? (
            <button type="button" className="button button--primary" onClick={onCreateBlock}>
              Bloquear horario
            </button>
          ) : null}
        </header>
        {blocks.length ? (
          <ul className="block-list">
            {blocks.map((block) => (
              <li key={block.id}>
                <span className={`block-mark block-mark--${block.kind}`} />
                <div>
                  <strong>{block.reason || "Sin detalle"}</strong>
                  <span>
                    {DateTime.fromISO(block.start_at).setZone(timezone).toFormat("dd LLL · HH:mm", {
                      locale: "es",
                    })}
                    {" – "}
                    {DateTime.fromISO(block.end_at).setZone(timezone).toFormat("HH:mm")}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <div className="empty-state empty-state--compact">
            <h3>Sin bloqueos en este rango</h3>
            <p>Feriados, ausencias y mantenimiento aparecerán acá.</p>
          </div>
        )}
      </section>
    </div>
  );
}
