-- 0004_party_model.down.sql

DROP TRIGGER IF EXISTS trg_party_contacts_updated_at ON party_contacts;
DROP TRIGGER IF EXISTS trg_parties_updated_at ON parties;

DROP TABLE IF EXISTS party_contacts;
DROP TABLE IF EXISTS party_classifications;
DROP TABLE IF EXISTS party_relationships;
DROP TABLE IF EXISTS party_roles;
DROP TABLE IF EXISTS party_agents;
DROP TABLE IF EXISTS party_organizations;
DROP TABLE IF EXISTS party_persons;
DROP TABLE IF EXISTS parties;
