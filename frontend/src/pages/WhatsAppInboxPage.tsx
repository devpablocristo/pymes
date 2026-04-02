import { LazyConfiguredCrudPage } from '../crud/lazyCrudPage';

export function WhatsAppInboxPage() {
  return <LazyConfiguredCrudPage resourceId="whatsappConversations" />;
}
