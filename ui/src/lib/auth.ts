import { resolveClerkBrowserConfig } from '@devpablocristo/platform-authn/providers/clerk';

const clerkConfig = resolveClerkBrowserConfig();

export const clerkEnabled = clerkConfig.enabled;
export const clerkPublishableKey = clerkConfig.publishableKey;
