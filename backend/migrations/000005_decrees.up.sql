CREATE TABLE decrees (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    decree_number TEXT UNIQUE NOT NULL, -- e.g. PALACE-DECREE-2026-001
    current_status publication_status NOT NULL DEFAULT 'draft',
    is_demo_content BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE decree_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    decree_id UUID NOT NULL REFERENCES decrees(id),
    version_number NUMERIC NOT NULL,
    title TEXT NOT NULL,
    full_text TEXT NOT NULL,
    issuing_authority TEXT NOT NULL,
    date_issued DATE,
    effective_date DATE,
    pdf_media_id UUID REFERENCES media(id),
    integrity_hash TEXT NOT NULL,
    status publication_status NOT NULL DEFAULT 'draft',
    is_correction BOOLEAN NOT NULL DEFAULT false,
    supersedes_version_id UUID REFERENCES decree_versions(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMPTZ,
    published_by UUID REFERENCES users(id),
    published_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(decree_id, version_number)
);
CREATE INDEX idx_decree_versions_decree ON decree_versions(decree_id);
CREATE INDEX idx_decree_versions_status ON decree_versions(status) WHERE status = 'published';

-- Database-level backstop: once a version row is published, its substantive
-- content can never be UPDATEd (only status may move published -> archived).
-- Corrections must INSERT a new decree_versions row instead.
CREATE OR REPLACE FUNCTION prevent_published_decree_version_edit()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status = 'published' THEN
        IF NEW.full_text IS DISTINCT FROM OLD.full_text
           OR NEW.title IS DISTINCT FROM OLD.title
           OR NEW.issuing_authority IS DISTINCT FROM OLD.issuing_authority
           OR NEW.integrity_hash IS DISTINCT FROM OLD.integrity_hash THEN
            RAISE EXCEPTION 'Cannot modify a published decree version; insert a correction version instead';
        END IF;
        IF NEW.status NOT IN ('published', 'archived') THEN
            RAISE EXCEPTION 'A published decree version can only transition to archived';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_published_decree_version_edit
    BEFORE UPDATE ON decree_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_published_decree_version_edit();

CREATE OR REPLACE FUNCTION prevent_decree_version_delete()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status = 'published' THEN
        RAISE EXCEPTION 'Cannot delete a published decree version';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_decree_version_delete
    BEFORE DELETE ON decree_versions
    FOR EACH ROW EXECUTE FUNCTION prevent_decree_version_delete();
