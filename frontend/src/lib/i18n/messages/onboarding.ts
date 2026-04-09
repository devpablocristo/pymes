import type { TranslationsByLanguage } from '../types';

export const onboardingMessages: TranslationsByLanguage = {
  es: {
    // Clerk
    'onboarding.clerk.sessionNotReady': 'Todavía se está cargando tu sesión. Esperá un momento y volvé a intentar.',
    'onboarding.clerk.organizationFailed':
      'No se pudo crear o activar la organización. Reintentá o volvé a iniciar sesión.',

    // Header
    'onboarding.header.title': 'Configurá tu espacio',
    'onboarding.header.subtitle': 'Unas preguntas rápidas para armar tu panel a medida.',

    // Step 1 — Tu negocio
    'onboarding.step1.title': 'Tu negocio',
    'onboarding.step1.businessName': '¿Cómo se llama tu negocio o actividad?',
    'onboarding.step1.businessNamePlaceholder': 'Ej: Clases de inglés, Estudio López, Mi emprendimiento...',
    'onboarding.step1.teamSize': '¿Cuántas personas trabajan?',
    'onboarding.step1.verticalGroup': '¿Qué tipo de negocio es?',
    'onboarding.step1.subVertical': '¿Qué tipo de taller?',

    // Team options
    'onboarding.team.solo': 'Solo yo',
    'onboarding.team.soloDesc': 'Trabajo por mi cuenta',
    'onboarding.team.small': '2 a 5',
    'onboarding.team.smallDesc': 'Equipo chico',
    'onboarding.team.medium': '6 a 20',
    'onboarding.team.mediumDesc': 'Equipo mediano',
    'onboarding.team.large': 'Más de 20',
    'onboarding.team.largeDesc': 'Empresa',

    // Vertical groups
    'onboarding.vertical.commercial': 'Solo comercial',
    'onboarding.vertical.commercialDesc': 'Ventas, stock y cobros',
    'onboarding.vertical.professionals': 'Profesionales / Docentes',
    'onboarding.vertical.professionalsDesc': 'Sesiones, alumnos y fichas',
    'onboarding.vertical.workshops': 'Talleres',
    'onboarding.vertical.workshopsDesc': 'Vehículos, órdenes de trabajo, reparaciones',
    'onboarding.vertical.bikeShop': 'Bicicletería',
    'onboarding.vertical.bikeShopDesc': 'Bicis, repuestos, órdenes de servicio',
    'onboarding.vertical.beauty': 'Belleza / Salón',
    'onboarding.vertical.beautyDesc': 'Equipo, servicios y agenda',
    'onboarding.vertical.restaurants': 'Bares / Restaurantes',
    'onboarding.vertical.restaurantsDesc': 'Salón, mesas y sesiones',

    // Step 2 — Tu actividad
    'onboarding.step2.title': 'Tu actividad',
    'onboarding.step2.sells': '¿Qué ofrecés?',
    'onboarding.step2.clientLabel': '¿Cómo les decís a las personas que te contratan?',
    'onboarding.step2.clientLabelCustom': 'otro...',
    'onboarding.step2.clientLabelCustomPlaceholder': '¿Cómo les decís?',
    'onboarding.step2.clientLabelCustomAria': 'Nombre personalizado para tus clientes',
    'onboarding.step2.scheduling': '¿Agendás turnos o sesiones con tus {clientLabel}?',
    'onboarding.step2.schedulingYes': 'Sí',
    'onboarding.step2.schedulingNo': 'No',
    'onboarding.step2.billing': '¿Querés llevar control de cobros y pagos?',
    'onboarding.step2.billingYes': 'Sí, quiero saber quién me debe y cuánto cobré',
    'onboarding.step2.billingNo': 'No, por ahora no',

    // Sells options
    'onboarding.sells.products': 'Productos',
    'onboarding.sells.productsDesc': 'Vendo cosas físicas, tengo stock',
    'onboarding.sells.services': 'Servicios',
    'onboarding.sells.servicesDesc': 'Cobro por hora, sesión o proyecto',
    'onboarding.sells.both': 'Ambos',
    'onboarding.sells.bothDesc': 'Productos y servicios',
    'onboarding.sells.unsure': 'Todavía no sé',
    'onboarding.sells.unsureDesc': 'Estoy explorando',

    // Client labels
    'onboarding.clientLabel.clientes': 'clientes',
    'onboarding.clientLabel.pacientes': 'pacientes',
    'onboarding.clientLabel.alumnos': 'alumnos',
    'onboarding.clientLabel.usuarios': 'usuarios',

    // Step 3 — Moneda y cobro
    'onboarding.step3.title': 'Moneda y cobro',
    'onboarding.step3.currency': '¿En qué moneda operás?',
    'onboarding.step3.paymentMethod': '¿Cómo cobrás principalmente?',

    // Currency options
    'onboarding.currency.ARS': 'Peso argentino (ARS)',
    'onboarding.currency.USD': 'Dólar (USD)',
    'onboarding.currency.EUR': 'Euro (EUR)',
    'onboarding.currency.BRL': 'Real (BRL)',
    'onboarding.currency.MXN': 'Peso mexicano (MXN)',
    'onboarding.currency.CLP': 'Peso chileno (CLP)',
    'onboarding.currency.COP': 'Peso colombiano (COP)',

    // Payment options
    'onboarding.payment.cash': 'Efectivo',
    'onboarding.payment.transfer': 'Transferencia',
    'onboarding.payment.card': 'Tarjeta',
    'onboarding.payment.mixed': 'Mixto (varios)',

    // Step 4 — Resumen
    'onboarding.step4.title': 'Todo listo',
    'onboarding.step4.intro': 'Vamos a configurar tu panel con esta información. Podés cambiarlo cuando quieras.',
    'onboarding.step4.business': 'Negocio',
    'onboarding.step4.team': 'Equipo',
    'onboarding.step4.verticalType': 'Tipo de negocio',
    'onboarding.step4.sells': 'Ofrecés',
    'onboarding.step4.clientLabel': 'Les decís',
    'onboarding.step4.scheduling': 'Agenda',
    'onboarding.step4.billing': 'Control de cobros',
    'onboarding.step4.currency': 'Moneda',
    'onboarding.step4.paymentMethod': 'Cobro',
    'onboarding.step4.yes': 'Sí',
    'onboarding.step4.no': 'No',

    // Navigation
    'onboarding.nav.back': 'Atrás',
    'onboarding.nav.next': 'Siguiente',
    'onboarding.nav.start': 'Empezar',
  },
  en: {
    'onboarding.clerk.sessionNotReady': 'Your session is still loading. Wait a moment and try again.',
    'onboarding.clerk.organizationFailed': 'Could not create or activate the organization. Try again or sign in again.',

    'onboarding.header.title': 'Set up your space',
    'onboarding.header.subtitle': 'A few quick questions to customize your dashboard.',

    'onboarding.step1.title': 'Your business',
    'onboarding.step1.businessName': 'What is the name of your business or activity?',
    'onboarding.step1.businessNamePlaceholder': 'E.g.: English classes, Studio López, My startup...',
    'onboarding.step1.teamSize': 'How many people work with you?',
    'onboarding.step1.verticalGroup': 'What type of business is it?',
    'onboarding.step1.subVertical': 'What type of workshop?',

    'onboarding.team.solo': 'Just me',
    'onboarding.team.soloDesc': 'I work on my own',
    'onboarding.team.small': '2 to 5',
    'onboarding.team.smallDesc': 'Small team',
    'onboarding.team.medium': '6 to 20',
    'onboarding.team.mediumDesc': 'Medium team',
    'onboarding.team.large': 'More than 20',
    'onboarding.team.largeDesc': 'Company',

    'onboarding.vertical.commercial': 'Commercial only',
    'onboarding.vertical.commercialDesc': 'Sales, stock and payments',
    'onboarding.vertical.professionals': 'Professionals / Teachers',
    'onboarding.vertical.professionalsDesc': 'Sessions, students and files',
    'onboarding.vertical.workshops': 'Workshops',
    'onboarding.vertical.workshopsDesc': 'Vehicles, work orders, repairs',
    'onboarding.vertical.bikeShop': 'Bike shop',
    'onboarding.vertical.bikeShopDesc': 'Bikes, parts, service orders',
    'onboarding.vertical.beauty': 'Beauty / Salon',
    'onboarding.vertical.beautyDesc': 'Team, services and bookings',
    'onboarding.vertical.restaurants': 'Bars / Restaurants',
    'onboarding.vertical.restaurantsDesc': 'Dining, tables and sessions',

    'onboarding.step2.title': 'Your activity',
    'onboarding.step2.sells': 'What do you offer?',
    'onboarding.step2.clientLabel': 'What do you call the people who hire you?',
    'onboarding.step2.clientLabelCustom': 'other...',
    'onboarding.step2.clientLabelCustomPlaceholder': 'What do you call them?',
    'onboarding.step2.clientLabelCustomAria': 'Custom name for your clients',
    'onboarding.step2.scheduling': 'Do you schedule bookings with your {clientLabel}?',
    'onboarding.step2.schedulingYes': 'Yes',
    'onboarding.step2.schedulingNo': 'No',
    'onboarding.step2.billing': 'Do you want to track payments and collections?',
    'onboarding.step2.billingYes': 'Yes, I want to know who owes me and how much I collected',
    'onboarding.step2.billingNo': 'No, not for now',

    'onboarding.sells.products': 'Products',
    'onboarding.sells.productsDesc': 'I sell physical goods, I have stock',
    'onboarding.sells.services': 'Services',
    'onboarding.sells.servicesDesc': 'I charge per hour, session or project',
    'onboarding.sells.both': 'Both',
    'onboarding.sells.bothDesc': 'Products and services',
    'onboarding.sells.unsure': "I don't know yet",
    'onboarding.sells.unsureDesc': "I'm exploring",

    'onboarding.clientLabel.clientes': 'clients',
    'onboarding.clientLabel.pacientes': 'patients',
    'onboarding.clientLabel.alumnos': 'students',
    'onboarding.clientLabel.usuarios': 'users',

    'onboarding.step3.title': 'Currency and payments',
    'onboarding.step3.currency': 'What currency do you operate in?',
    'onboarding.step3.paymentMethod': 'How do you mainly get paid?',

    'onboarding.currency.ARS': 'Argentine peso (ARS)',
    'onboarding.currency.USD': 'US dollar (USD)',
    'onboarding.currency.EUR': 'Euro (EUR)',
    'onboarding.currency.BRL': 'Brazilian real (BRL)',
    'onboarding.currency.MXN': 'Mexican peso (MXN)',
    'onboarding.currency.CLP': 'Chilean peso (CLP)',
    'onboarding.currency.COP': 'Colombian peso (COP)',

    'onboarding.payment.cash': 'Cash',
    'onboarding.payment.transfer': 'Bank transfer',
    'onboarding.payment.card': 'Card',
    'onboarding.payment.mixed': 'Mixed (various)',

    'onboarding.step4.title': 'All set',
    'onboarding.step4.intro': "We'll set up your dashboard with this info. You can change it anytime.",
    'onboarding.step4.business': 'Business',
    'onboarding.step4.team': 'Team',
    'onboarding.step4.verticalType': 'Business type',
    'onboarding.step4.sells': 'You offer',
    'onboarding.step4.clientLabel': 'You call them',
    'onboarding.step4.scheduling': 'Schedule',
    'onboarding.step4.billing': 'Payment tracking',
    'onboarding.step4.currency': 'Currency',
    'onboarding.step4.paymentMethod': 'Payment method',
    'onboarding.step4.yes': 'Yes',
    'onboarding.step4.no': 'No',

    'onboarding.nav.back': 'Back',
    'onboarding.nav.next': 'Next',
    'onboarding.nav.start': 'Start',
  },
};
