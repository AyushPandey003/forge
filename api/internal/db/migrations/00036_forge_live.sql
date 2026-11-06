-- +goose Up
-- Durable invalidation cursors, not a business-event log. Incrementing rows
-- serializes concurrent updates per user; sequence allocation alone would not
-- provide commit-ordered cursors. No sensitive event payload is exposed.
CREATE TABLE forge_live_revisions (
  user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision BIGINT NOT NULL DEFAULT 0
);

-- +goose StatementBegin
CREATE FUNCTION forge_live_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  sid TEXT;
  old_user TEXT;
  did TEXT;
BEGIN
  IF TG_TABLE_NAME = 'documents' THEN
    did := NEW.id;
  ELSIF TG_TABLE_NAME = 'forge_solutions' THEN
    sid := NEW.id;
  ELSE
    IF TG_OP = 'DELETE' THEN
      sid := OLD.solution_id;
    ELSE
      sid := NEW.solution_id;
    END IF;
    IF TG_TABLE_NAME = 'forge_members' AND TG_OP = 'DELETE' THEN
      old_user := OLD.user_id;
    END IF;
  END IF;

  INSERT INTO forge_live_revisions(user_id, revision)
  SELECT DISTINCT user_id, 1 FROM (
    SELECT s.created_by AS user_id FROM forge_solutions s
    WHERE s.id = sid OR EXISTS (SELECT 1 FROM forge_artifacts a WHERE a.solution_id=s.id AND a.document_id=did)
    UNION
    SELECT m.user_id FROM forge_members m
    WHERE m.solution_id = sid OR EXISTS (SELECT 1 FROM forge_artifacts a WHERE a.solution_id=m.solution_id AND a.document_id=did)
    UNION
    SELECT old_user WHERE old_user IS NOT NULL
  ) recipients ORDER BY user_id
  ON CONFLICT(user_id) DO UPDATE SET revision=forge_live_revisions.revision+1;
  RETURN NULL;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER forge_live_solution AFTER INSERT OR UPDATE ON forge_solutions FOR EACH ROW EXECUTE FUNCTION forge_live_changed();
CREATE TRIGGER forge_live_member AFTER INSERT OR UPDATE OR DELETE ON forge_members FOR EACH ROW EXECUTE FUNCTION forge_live_changed();
CREATE TRIGGER forge_live_artifact AFTER INSERT OR UPDATE OR DELETE ON forge_artifacts FOR EACH ROW EXECUTE FUNCTION forge_live_changed();
CREATE TRIGGER forge_live_approval AFTER INSERT OR UPDATE OR DELETE ON forge_approvals FOR EACH ROW EXECUTE FUNCTION forge_live_changed();
CREATE TRIGGER forge_live_event AFTER INSERT ON forge_events FOR EACH ROW EXECUTE FUNCTION forge_live_changed();
CREATE TRIGGER forge_live_document AFTER UPDATE ON documents FOR EACH ROW WHEN (OLD.current_version_id IS DISTINCT FROM NEW.current_version_id OR OLD.title IS DISTINCT FROM NEW.title) EXECUTE FUNCTION forge_live_changed();

-- +goose Down
DROP TRIGGER forge_live_document ON documents;
DROP TRIGGER forge_live_event ON forge_events;
DROP TRIGGER forge_live_approval ON forge_approvals;
DROP TRIGGER forge_live_artifact ON forge_artifacts;
DROP TRIGGER forge_live_member ON forge_members;
DROP TRIGGER forge_live_solution ON forge_solutions;
DROP FUNCTION forge_live_changed();
DROP TABLE forge_live_revisions;
