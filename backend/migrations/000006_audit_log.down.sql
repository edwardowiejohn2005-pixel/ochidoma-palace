DROP TRIGGER IF EXISTS trg_reject_audit_log_delete ON audit_logs;
DROP TRIGGER IF EXISTS trg_reject_audit_log_update ON audit_logs;
DROP FUNCTION IF EXISTS reject_audit_log_mutation();
DROP TABLE IF EXISTS audit_logs;
