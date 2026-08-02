import type { Pool, PoolClient } from "pg";
import {
  CredentialError,
  type CredentialEnvironment,
  type PointOfSale,
} from "./usecases/domain/credential.js";
import type {
  CertificateInspection,
  CredentialRepository,
  ArtifactRepository,
  SealedValue,
  StoredAccessTicket,
  StoredArtifact,
  StoredCredential,
  TicketRepository,
} from "./usecases.js";
import {
  credentialFromRow,
  accessTicketFromRow,
  isUniqueViolation,
  pointOfSaleFromRow,
} from "./repository/helpers/mappers.js";
import type {
  AccessTicketRow,
  CredentialRow,
  PointOfSaleRow,
} from "./repository/models/rows.js";

export class PostgresCredentialRepository
  implements CredentialRepository, TicketRepository, ArtifactRepository
{
  constructor(private readonly pool: Pool) {}

  async findByIdempotency(
    organizationId: string,
    idempotencyKey: string,
  ): Promise<StoredCredential | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<CredentialRow>(
        `SELECT * FROM fiscal.credentials
          WHERE organization_id=$1 AND idempotency_key=$2`,
        [organizationId, idempotencyKey],
      );
      const row = result.rows[0];
      return row === undefined ? undefined : credentialFromRow(row);
    });
  }

  async insertPending(record: StoredCredential): Promise<StoredCredential> {
    try {
      return await this.withOrganization(record.organizationId, async (client) => {
        const result = await client.query<CredentialRow>(
          `INSERT INTO fiscal.credentials
             (organization_id,credential_id,cuit,environment,legal_name,common_name,
              status,idempotency_key,request_hash,csr_pem,encrypted_private_key,
              version,created_at,updated_at)
           VALUES($1,$2,$3,$4,$5,$6,'pending_certificate',$7,$8,$9,$10,1,$11,$11)
           RETURNING *`,
          [
            record.organizationId,
            record.id,
            record.cuit,
            record.environment,
            record.legalName,
            record.commonName,
            record.idempotencyKey,
            record.requestHash,
            record.csrPem,
            record.encryptedPrivateKey,
            record.createdAt,
          ],
        );
        return credentialFromRow(result.rows[0]!);
      });
    } catch (error) {
      if (isUniqueViolation(error)) {
        throw new CredentialError("CREDENTIAL_VERSION_CONFLICT");
      }
      throw error;
    }
  }

  async find(
    organizationId: string,
    credentialId: string,
  ): Promise<StoredCredential | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<CredentialRow>(
        `SELECT * FROM fiscal.credentials
          WHERE organization_id=$1 AND credential_id=$2`,
        [organizationId, credentialId],
      );
      const row = result.rows[0];
      return row === undefined ? undefined : credentialFromRow(row);
    });
  }

  async activate(
    organizationId: string,
    credentialId: string,
    expectedVersion: number,
    certificate: SealedValue,
    inspection: CertificateInspection,
  ): Promise<StoredCredential> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<CredentialRow>(
        `UPDATE fiscal.credentials
            SET encrypted_certificate=$4,
                certificate_fingerprint=$5,
                certificate_valid_from=$6,
                certificate_expires_at=$7,
                certificate_serial_number=$8,
                status='ready',
                version=version+1,
                updated_at=now()
          WHERE organization_id=$1 AND credential_id=$2 AND version=$3
          RETURNING *`,
        [
          organizationId,
          credentialId,
          expectedVersion,
          certificate,
          inspection.fingerprint,
          inspection.validFrom,
          inspection.expiresAt,
          inspection.serialNumber,
        ],
      );
      if (result.rows[0] === undefined) {
        throw new CredentialError("CREDENTIAL_VERSION_CONFLICT");
      }
      return credentialFromRow(result.rows[0]);
    });
  }

  async hasValidatedPointOfSale(
    organizationId: string,
    environment: CredentialEnvironment,
    cuit: string,
  ): Promise<boolean> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<{ validated: boolean }>(
        `SELECT EXISTS(
           SELECT 1
             FROM fiscal.credentials AS credential
             JOIN fiscal.points_of_sale AS point
               ON point.organization_id=credential.organization_id
              AND point.credential_id=credential.id
              AND point.environment=credential.environment
            WHERE credential.organization_id=$1
              AND credential.environment=$2
              AND credential.cuit=$3
              AND credential.status='ready'
              AND credential.certificate_expires_at > now()
              AND point.enabled=true
              AND point.validated_at IS NOT NULL
         ) AS validated`,
        [organizationId, environment, cuit],
      );
      return result.rows[0]?.validated === true;
    });
  }

  async upsertPointOfSale(pointOfSale: PointOfSale): Promise<PointOfSale> {
    return this.withOrganization(pointOfSale.organizationId, async (client) => {
      const result = await client.query<PointOfSaleRow>(
        `INSERT INTO fiscal.points_of_sale
           (organization_id,credential_id,environment,point_of_sale,enabled,validated_at,updated_at)
         VALUES($1,$2,$3,$4,$5,$6,now())
         ON CONFLICT (organization_id,credential_id,environment,point_of_sale)
         DO UPDATE SET enabled=EXCLUDED.enabled,
                       validated_at=COALESCE(EXCLUDED.validated_at,fiscal.points_of_sale.validated_at),
                       updated_at=now()
         RETURNING organization_id,credential_id,environment,point_of_sale,enabled,validated_at`,
        [
          pointOfSale.organizationId,
          pointOfSale.credentialId,
          pointOfSale.environment,
          pointOfSale.number,
          pointOfSale.enabled,
          pointOfSale.validatedAt ?? null,
        ],
      );
      return pointOfSaleFromRow(result.rows[0]!);
    });
  }

  async findPointOfSale(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    number: number,
  ): Promise<PointOfSale | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<PointOfSaleRow>(
        `SELECT organization_id,credential_id,environment,point_of_sale,enabled,validated_at
           FROM fiscal.points_of_sale
          WHERE organization_id=$1
            AND credential_id=$2
            AND environment=$3
            AND point_of_sale=$4`,
        [organizationId, credentialId, environment, number],
      );
      const row = result.rows[0];
      return row === undefined ? undefined : pointOfSaleFromRow(row);
    });
  }

  async findTicket(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    service: string,
  ): Promise<StoredAccessTicket | undefined> {
    return this.withOrganization(organizationId, async (client) => {
      const result = await client.query<AccessTicketRow>(
        `SELECT organization_id,credential_id,environment,service,encrypted_ticket,expires_at
           FROM fiscal.wsaa_tickets
          WHERE organization_id=$1
            AND credential_id=$2
            AND environment=$3
            AND service=$4`,
        [organizationId, credentialId, environment, service],
      );
      const row = result.rows[0];
      return row === undefined ? undefined : accessTicketFromRow(row);
    });
  }

  async saveTicket(ticket: StoredAccessTicket): Promise<void> {
    await this.withOrganization(ticket.organizationId, (client) =>
      client.query(
        `INSERT INTO fiscal.wsaa_tickets
           (organization_id,credential_id,environment,service,encrypted_ticket,expires_at,created_at,updated_at)
         VALUES($1,$2,$3,$4,$5,$6,now(),now())
         ON CONFLICT (organization_id,credential_id,environment,service)
         DO UPDATE SET encrypted_ticket=EXCLUDED.encrypted_ticket,
                       expires_at=EXCLUDED.expires_at,
                       updated_at=now()`,
        [
          ticket.organizationId,
          ticket.credentialId,
          ticket.environment,
          ticket.service,
          ticket.encryptedTicket,
          ticket.expiresAt,
        ],
      ),
    );
  }

  async deleteTicket(
    organizationId: string,
    credentialId: string,
    environment: CredentialEnvironment,
    service: string,
  ): Promise<void> {
    await this.withOrganization(organizationId, (client) =>
      client.query(
        `DELETE FROM fiscal.wsaa_tickets
          WHERE organization_id=$1
            AND credential_id=$2
            AND environment=$3
            AND service=$4`,
        [organizationId, credentialId, environment, service],
      ),
    );
  }

  async saveArtifact(artifact: StoredArtifact): Promise<void> {
    await this.withOrganization(artifact.organizationId, (client) =>
      client.query(
        `INSERT INTO fiscal.encrypted_artifacts
           (organization_id,artifact_id,request_id,kind,encrypted_payload,created_at)
         VALUES($1,$2,$3,$4,$5,now())
         ON CONFLICT (organization_id,artifact_id) DO NOTHING`,
        [
          artifact.organizationId,
          artifact.artifactId,
          artifact.requestId,
          artifact.kind,
          artifact.encryptedPayload,
        ],
      ),
    );
  }

  private async withOrganization<T>(
    organizationId: string,
    operation: (client: PoolClient) => Promise<T>,
  ): Promise<T> {
    if (!/^org_[A-Za-z0-9_-]+$/.test(organizationId)) {
      throw new CredentialError("VALIDATION_ERROR", "invalid organization");
    }
    const client = await this.pool.connect();
    try {
      await client.query("BEGIN");
      await client.query(
        "SELECT set_config('app.organization_id',$1,true)",
        [organizationId],
      );
      const result = await operation(client);
      await client.query("COMMIT");
      return result;
    } catch (error) {
      await client.query("ROLLBACK").catch(() => undefined);
      throw error;
    } finally {
      client.release();
    }
  }
}
