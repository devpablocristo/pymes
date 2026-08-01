import { DateTime } from "luxon";
import { type FormEvent, useState } from "react";
import { useSchedulingGateway } from "../api/GatewayContext";
import { errorMessage } from "../api/errors";

type Purpose = "confirm" | "cancel" | "reschedule" | "accept_waitlist";

export function PublicActionPage({ token, search }: { token: string; search: string }) {
  const gateway = useSchedulingGateway();
  const params = new URLSearchParams(search);
  const purpose = (params.get("purpose") ?? "confirm") as Purpose;
  const version = Number(params.get("version") ?? "1");
  const [startAt, setStartAt] = useState("");
  const [reason, setReason] = useState("");
  const [state, setState] = useState<"idle" | "pending" | "done" | "error">("idle");
  const [message, setMessage] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setState("pending");
    try {
      await gateway.consumePublicAction(token, {
        purpose,
        expected_version: version,
        ...(startAt
          ? {
              start_at:
                DateTime.fromFormat(startAt, "yyyy-MM-dd'T'HH:mm", {
                  zone: "America/Argentina/Buenos_Aires",
                })
                  .toUTC()
                  .toISO() ?? startAt,
            }
          : {}),
        ...(reason ? { reason } : {}),
      });
      setState("done");
    } catch (error) {
      setMessage(errorMessage(error));
      setState("error");
    }
  }

  const labels: Record<Purpose, { title: string; action: string }> = {
    confirm: { title: "Confirmar turno", action: "Confirmar" },
    cancel: { title: "Cancelar turno", action: "Cancelar turno" },
    reschedule: { title: "Elegir un nuevo horario", action: "Reprogramar" },
    accept_waitlist: { title: "Aceptar el lugar disponible", action: "Aceptar lugar" },
  };

  return (
    <main className="public-action" id="main-content">
      <div className="public-action__card">
        <span className="public-brand-mark">P</span>
        {state === "done" ? (
          <>
            <p className="eyebrow">Acción registrada</p>
            <h1>Listo, guardamos el cambio</h1>
            <p>Podés cerrar esta ventana.</p>
          </>
        ) : (
          <>
            <p className="eyebrow">Gestión de reserva</p>
            <h1>{labels[purpose].title}</h1>
            <p>Este enlace es personal, de propósito único y se consume de forma segura.</p>
            {state === "error" ? (
              <div className="inline-alert" role="alert">
                {message}
              </div>
            ) : null}
            <form onSubmit={(event) => void submit(event)}>
              {purpose === "reschedule" ? (
                <label className="field">
                  <span>Nuevo horario</span>
                  <input
                    type="datetime-local"
                    value={startAt}
                    onChange={(event) => setStartAt(event.target.value)}
                    required
                  />
                </label>
              ) : null}
              {purpose === "cancel" ? (
                <label className="field">
                  <span>Motivo</span>
                  <textarea value={reason} onChange={(event) => setReason(event.target.value)} minLength={3} required />
                </label>
              ) : null}
              <button type="submit" className="button button--primary" disabled={state === "pending"}>
                {state === "pending" ? "Guardando…" : labels[purpose].action}
              </button>
            </form>
          </>
        )}
      </div>
    </main>
  );
}
