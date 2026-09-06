DROP TRIGGER IF EXISTS trg_prevent_decree_version_delete ON decree_versions;
DROP FUNCTION IF EXISTS prevent_decree_version_delete();
DROP TRIGGER IF EXISTS trg_prevent_published_decree_version_edit ON decree_versions;
DROP FUNCTION IF EXISTS prevent_published_decree_version_edit();
DROP TABLE IF EXISTS decree_versions;
DROP TABLE IF EXISTS decrees;
