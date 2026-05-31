package store

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

type sqliteIcecastStore struct {
	db *sql.DB
}

// scanIcecastServer reads an icecast_servers row into a model.IcecastServer.
func scanIcecastServer(row interface {
	Scan(...any) error
}) (*model.IcecastServer, error) {
	var s model.IcecastServer
	var createdAt, updatedAt string
	err := row.Scan(
		&s.ID,
		&s.Name,
		&s.Host,
		&s.Port,
		&s.Protocol,
		&s.SourceUser,
		&s.SourcePass,
		&s.Enabled,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// icecastServerSelectCols is the standard SELECT column list for icecast_servers.
const icecastServerSelectCols = `id, name, host, port, protocol, source_user, source_pass, enabled, created_at, updated_at`

func (s *sqliteIcecastStore) CreateServer(ctx context.Context, srv *model.IcecastServer) (int64, error) {
	const q = `INSERT INTO icecast_servers (name, host, port, protocol, source_user, source_pass, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	res, err := s.db.ExecContext(ctx, q,
		srv.Name, srv.Host, srv.Port, srv.Protocol, srv.SourceUser, srv.SourcePass, srv.Enabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *sqliteIcecastStore) GetServer(ctx context.Context, id int64) (*model.IcecastServer, error) {
	q := `SELECT ` + icecastServerSelectCols + ` FROM icecast_servers WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	srv, err := scanIcecastServer(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return srv, err
}

func (s *sqliteIcecastStore) ListServers(ctx context.Context) ([]*model.IcecastServer, error) {
	q := `SELECT ` + icecastServerSelectCols + ` FROM icecast_servers ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*model.IcecastServer
	for rows.Next() {
		srv, err := scanIcecastServer(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, srv)
	}
	return servers, rows.Err()
}

func (s *sqliteIcecastStore) UpdateServer(ctx context.Context, srv *model.IcecastServer) error {
	const q = `UPDATE icecast_servers SET
		name = ?, host = ?, port = ?, protocol = ?, source_user = ?, source_pass = ?,
		enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q,
		srv.Name, srv.Host, srv.Port, srv.Protocol, srv.SourceUser, srv.SourcePass,
		srv.Enabled, srv.ID)
	return err
}

func (s *sqliteIcecastStore) DeleteServer(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM icecast_servers WHERE id = ?`, id)
	return err
}

// scanIcecastMount reads an icecast_mounts row into a model.IcecastMount.
func scanIcecastMount(row interface {
	Scan(...any) error
}) (*model.IcecastMount, error) {
	var m model.IcecastMount
	var createdAt, updatedAt string
	err := row.Scan(
		&m.ID,
		&m.ServerID,
		&m.MountPath,
		&m.Name,
		&m.Description,
		&m.Scope,
		&m.ArtistID,
		&m.Genre,
		&m.Format,
		&m.BitRate,
		&m.Shuffle,
		&m.Enabled,
		&m.Status,
		&m.CurrentSong,
		&m.LastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &m, nil
}

// icecastMountSelectCols is the standard SELECT column list for icecast_mounts.
const icecastMountSelectCols = `id, server_id, mount_path, name, description, scope, artist_id, genre,
	format, bit_rate, shuffle, enabled, status, current_song, last_error, created_at, updated_at`

func (s *sqliteIcecastStore) CreateMount(ctx context.Context, m *model.IcecastMount) (int64, error) {
	const q = `INSERT INTO icecast_mounts
		(server_id, mount_path, name, description, scope, artist_id, genre, format, bit_rate, shuffle, enabled, status, current_song, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	res, err := s.db.ExecContext(ctx, q,
		m.ServerID, m.MountPath, m.Name, m.Description, m.Scope, m.ArtistID, m.Genre,
		m.Format, m.BitRate, m.Shuffle, m.Enabled, m.Status, m.CurrentSong, m.LastError)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *sqliteIcecastStore) GetMount(ctx context.Context, id int64) (*model.IcecastMount, error) {
	q := `SELECT ` + icecastMountSelectCols + ` FROM icecast_mounts WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	m, err := scanIcecastMount(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (s *sqliteIcecastStore) ListMounts(ctx context.Context) ([]*model.IcecastMount, error) {
	q := `SELECT ` + icecastMountSelectCols + ` FROM icecast_mounts ORDER BY server_id, name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectMounts(rows)
}

func (s *sqliteIcecastStore) ListMountsByServer(ctx context.Context, serverID int64) ([]*model.IcecastMount, error) {
	q := `SELECT ` + icecastMountSelectCols + ` FROM icecast_mounts WHERE server_id = ? ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectMounts(rows)
}

func (s *sqliteIcecastStore) UpdateMount(ctx context.Context, m *model.IcecastMount) error {
	const q = `UPDATE icecast_mounts SET
		server_id = ?, mount_path = ?, name = ?, description = ?, scope = ?, artist_id = ?,
		genre = ?, format = ?, bit_rate = ?, shuffle = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q,
		m.ServerID, m.MountPath, m.Name, m.Description, m.Scope, m.ArtistID,
		m.Genre, m.Format, m.BitRate, m.Shuffle, m.Enabled, m.ID)
	return err
}

func (s *sqliteIcecastStore) UpdateMountStatus(ctx context.Context, id int64, status model.MountStatus, currentSong, lastErr string) error {
	const q = `UPDATE icecast_mounts SET
		status = ?, current_song = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`
	_, err := s.db.ExecContext(ctx, q, status, currentSong, lastErr, id)
	return err
}

func (s *sqliteIcecastStore) DeleteMount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM icecast_mounts WHERE id = ?`, id)
	return err
}

// collectMounts iterates over a *sql.Rows result set and returns all mounts.
func collectMounts(rows *sql.Rows) ([]*model.IcecastMount, error) {
	var mounts []*model.IcecastMount
	for rows.Next() {
		m, err := scanIcecastMount(rows)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, m)
	}
	return mounts, rows.Err()
}
