-- +goose Up
-- Supabase memberi peran anon dan authenticated akses penuh ke tabel di schema public, dan anon key
-- ikut terkirim ke browser. Kalau Data API menyala, siapa pun bisa membaca dan mengubah tabel lewat REST.
-- Hanya API Go yang memakai database, jadi akses itu dicabut dan RLS dinyalakan tanpa policy.
-- API Go tetap bisa membaca karena memakai pemilik tabel. Di Postgres biasa tanpa peran itu, hanya RLS yang berubah.
-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', t);
    END LOOP;
    IF (SELECT count(*) FROM pg_roles WHERE rolname IN ('anon', 'authenticated')) = 2 THEN
        REVOKE ALL ON ALL TABLES IN SCHEMA public FROM anon, authenticated;
        REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM anon, authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON TABLES FROM anon, authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE ALL ON SEQUENCES FROM anon, authenticated;
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    t text;
BEGIN
    FOR t IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I DISABLE ROW LEVEL SECURITY', t);
    END LOOP;
    IF (SELECT count(*) FROM pg_roles WHERE rolname IN ('anon', 'authenticated')) = 2 THEN
        GRANT ALL ON ALL TABLES IN SCHEMA public TO anon, authenticated;
        GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO anon, authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO anon, authenticated;
        ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO anon, authenticated;
    END IF;
END $$;
-- +goose StatementEnd
