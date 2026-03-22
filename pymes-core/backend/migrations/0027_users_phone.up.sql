-- Teléfono opcional del usuario SaaS (perfil de producto); el dominio core User aún no expone phone en JSON.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS phone text NOT NULL DEFAULT '';
