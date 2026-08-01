export interface InternalIdentity {
  issuer: string;
  subject: string;
  organizationId: string;
  actorId?: string;
  delegatedActorId?: string;
  roles: string[];
  requestId: string;
  correlationId: string;
  tokenId: string;
}

export interface InternalAuthorizer {
  authorize(
    authorization: string | undefined,
    audience: "fiscal",
    expectedOrganizationId?: string,
    expectedCorrelationId?: string,
  ): Promise<InternalIdentity>;
}
