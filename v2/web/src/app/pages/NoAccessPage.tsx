import { useProductAuth } from "../../auth/AuthContext";

export function NoAccessPage() {
  const auth = useProductAuth();

  return (
    <main className="no-access">
      <img src="/assets/iso.svg" alt="" />
      <p className="no-access__eyebrow">Acceso restringido</p>
      <h1>Todavía no tenés una organización disponible.</h1>
      <p>
        Pedile a un owner o administrador que te envíe una invitación al email con
        el que ingresaste.
      </p>
      <button className="button button--primary" type="button" onClick={() => void auth.signOut()}>
        Cerrar sesión
      </button>
    </main>
  );
}
