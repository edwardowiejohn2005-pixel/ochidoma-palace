CREATE TABLE articles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    section TEXT NOT NULL, -- heritage | history
    category_id INT REFERENCES categories(id),
    title TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    body TEXT NOT NULL,
    sources TEXT,
    author_id UUID REFERENCES users(id),
    status publication_status NOT NULL DEFAULT 'draft',
    is_demo_content BOOLEAN NOT NULL DEFAULT false,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_articles_status ON articles(status) WHERE status = 'published';
CREATE INDEX idx_articles_section ON articles(section);

CREATE TABLE foods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    local_name TEXT,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    ingredients TEXT,
    preparation TEXT,
    cultural_significance TEXT,
    region TEXT,
    status publication_status NOT NULL DEFAULT 'draft',
    is_demo_content BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_foods_status ON foods(status) WHERE status = 'published';

CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    description TEXT,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ,
    location TEXT,
    organizer TEXT,
    category TEXT,
    status TEXT NOT NULL DEFAULT 'upcoming', -- upcoming | ongoing | completed
    is_demo_content BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_events_status ON events(status);
CREATE INDEX idx_events_starts_at ON events(starts_at);
