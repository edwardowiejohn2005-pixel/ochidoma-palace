CREATE TABLE announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reference_number TEXT UNIQUE,
    title TEXT NOT NULL,
    category TEXT,
    content TEXT NOT NULL,
    status publication_status NOT NULL DEFAULT 'draft',
    is_official BOOLEAN NOT NULL DEFAULT true,
    is_demo_content BOOLEAN NOT NULL DEFAULT false,
    author_id UUID REFERENCES users(id),
    published_by UUID REFERENCES users(id),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_announcements_status ON announcements(status) WHERE status = 'published';

-- Reject silent edits to already-published announcements at the DB layer.
CREATE OR REPLACE FUNCTION prevent_published_announcement_edit()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.status = 'published' AND NEW.status = 'published'
       AND (NEW.content IS DISTINCT FROM OLD.content OR NEW.title IS DISTINCT FROM OLD.title) THEN
        RAISE EXCEPTION 'Cannot silently modify a published announcement; create a correction instead';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_prevent_published_announcement_edit
    BEFORE UPDATE ON announcements
    FOR EACH ROW EXECUTE FUNCTION prevent_published_announcement_edit();
