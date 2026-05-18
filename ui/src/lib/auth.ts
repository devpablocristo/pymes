import { resolveClerkBrowserConfig } from '@devpablocristo/core-authn/providers/clerk';

const clerkConfig = resolveClerkBrowserConfig();

export const clerkEnabled = clerkConfig.enabled;
export const clerkPublishableKey = clerkConfig.publishableKey;
