package forge

import "context"

// LiveRevision is a durable, commit-ordered invalidation cursor for one user.
// A client catching up needs only a fresh authorized snapshot, not every event.
func (s *Store) LiveRevision(ctx context.Context, userID string) (int64, error) {
	var revision int64
	err := s.db.QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM forge_live_revisions WHERE user_id=$1),0)`, userID).Scan(&revision)
	return revision, err
}
