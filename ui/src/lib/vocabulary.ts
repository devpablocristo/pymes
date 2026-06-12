import { getTenantProfile } from './tenantProfile';

const PLURAL_MAP: Record<string, string> = {
  alumnos: 'alumno',
  pacientes: 'paciente',
  usuarios: 'usuario',
  clientes: 'cliente',
};

function singularOf(plural: string): string {
  return PLURAL_MAP[plural] ?? plural.replace(/s$/, '');
}

// Replacements: [pattern, singular?, replacement builder]
// Built lazily from profile
function buildReplacements(): Array<{ regex: RegExp; replacer: (match: string) => string }> {
  const profile = getTenantProfile();
  if (!profile || !profile.clientLabel || profile.clientLabel === 'clientes') return [];

  const plural = profile.clientLabel;
  const singular = singularOf(plural);

  function matchCase(original: string, replacement: string): string {
    if (original[0] === original[0].toUpperCase()) {
      return replacement.charAt(0).toUpperCase() + replacement.slice(1);
    }
    return replacement;
  }

  return [
    { regex: /\bclientes\b/gi, replacer: (m: string) => matchCase(m, plural) },
    { regex: /\bcliente\b/gi, replacer: (m: string) => matchCase(m, singular) },
  ];
}

let cachedLabel: string | null = null;
let cachedReplacements: Array<{ regex: RegExp; replacer: (match: string) => string }> = [];

function getReplacements() {
  const profile = getTenantProfile();
  const label = profile?.clientLabel ?? null;
  if (label !== cachedLabel) {
    cachedLabel = label;
    cachedReplacements = buildReplacements();
  }
  return cachedReplacements;
}

export function vocab(text: string): string {
  const replacements = getReplacements();
  if (replacements.length === 0) return text;

  let result = text;
  for (const { regex, replacer } of replacements) {
    result = result.replace(regex, replacer);
  }
  return result;
}
