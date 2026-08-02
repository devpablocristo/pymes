export interface JWTHeader {
  alg?: unknown;
  typ?: unknown;
  kid?: unknown;
}

export interface InternalClaims {
  iss?: unknown;
  aud?: unknown;
  sub?: unknown;
  org_id?: unknown;
  roles?: unknown;
  request_id?: unknown;
  correlation_id?: unknown;
  actor_id?: unknown;
  delegated_actor_id?: unknown;
  jti?: unknown;
  kid?: unknown;
  iat?: unknown;
  exp?: unknown;
}

export interface Ed25519JWK {
  kty?: unknown;
  crv?: unknown;
  alg?: unknown;
  use?: unknown;
  key_ops?: unknown;
  kid?: unknown;
  x?: unknown;
}

export interface JWKS {
  keys?: unknown;
}
