import { CrudPage } from '../components/CrudPage';
import { vocab } from '../lib/vocabulary';

type Customer = {
  id: string;
  name: string;
  email: string;
  phone: string;
  notes: string;
  tags?: string[];
  type?: string;
};

export function CustomersPage() {
  return (
    <CrudPage<Customer>
      basePath="/v1/customers"
      label={vocab('cliente')}
      labelPlural={vocab('clientes')}
      labelPluralCap={vocab('Clientes')}
      columns={[
        { key: 'name', header: 'Nombre', className: 'cell-name' },
        { key: 'email', header: 'Email' },
        { key: 'phone', header: 'Telefono' },
        { key: 'notes', header: 'Notas', className: 'cell-notes' },
      ]}
      formFields={[
        { key: 'name', label: 'Nombre', required: true, placeholder: `Nombre del ${vocab('cliente')}` },
        { key: 'email', label: 'Email', type: 'email', placeholder: 'email@ejemplo.com' },
        { key: 'phone', label: 'Telefono', type: 'tel', placeholder: '+54 11 1234-5678' },
        { key: 'notes', label: 'Notas', type: 'textarea', placeholder: 'Notas internas...', fullWidth: true },
      ]}
      searchText={(c) => [c.name, c.email, c.phone, c.notes].filter(Boolean).join(' ')}
      toFormValues={(c) => ({ name: c.name ?? '', email: c.email ?? '', phone: c.phone ?? '', notes: c.notes ?? '' })}
      toBody={(v) => ({ name: v.name, email: v.email || undefined, phone: v.phone || undefined, notes: v.notes || undefined })}
      isValid={(v) => (v.name ?? '').trim().length >= 2}
    />
  );
}
