export interface InternalIdentity {
  issuer: string;
  subject: string;
  organizationId: string;
  roles: string[];
  requestId: string;
  tokenId: string;
}

export interface InternalAuthorizer {
  authorize(
    authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
  ): Promise<InternalIdentity>;
}
