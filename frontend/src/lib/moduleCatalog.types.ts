export type ModuleRuntimeContext = {
  orgId: string;
  today: string;
  monthStart: string;
};

export type ValueResolver = string | ((ctx: ModuleRuntimeContext) => string);

export type ModuleField = {
  name: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  location?: 'path' | 'query' | 'body';
  type?: 'text' | 'number' | 'textarea' | 'date' | 'select' | 'json';
  defaultValue?: ValueResolver;
  options?: Array<{ label: string; value: string }>;
};

export type ModuleDataset = {
  id: string;
  title: string;
  description: string;
  path: string;
  fields?: ModuleField[];
  autoLoad?: boolean;
};

export type ModuleAction = {
  id: string;
  title: string;
  description: string;
  path: string;
  method: 'GET' | 'POST' | 'PUT' | 'DELETE';
  group?: string;
  fields?: ModuleField[];
  response?: 'json' | 'download' | 'none';
  submitLabel?: string;
  sendEmptyBody?: boolean;
};

export type ModuleDefinition = {
  id: string;
  title: string;
  navLabel: string;
  summary: string;
  group: string;
  icon: string;
  badge?: string;
  datasets?: ModuleDataset[];
  actions?: ModuleAction[];
  actionGroupOrder?: string[];
  actionGroupLabels?: Record<string, string>;
  notes?: string[];
  helpIntro?: string;
};
