import { NavLink } from "react-router-dom";

export function AdminDirectoryTabs() {
  return (
    <nav className="directory-tabs" aria-label="Usuarios y Tenants">
      <NavLink to="/admin/users">Usuarios</NavLink>
      <NavLink to="/admin/tenants">Tenants</NavLink>
      <NavLink to="/admin/invitations">Invitaciones</NavLink>
    </nav>
  );
}
