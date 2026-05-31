package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/local/cassonic/src/server/model"
)

type sqliteMusicStore struct {
	db *sql.DB
}

// scanLibrary reads a libraries row into a model.Library.
func scanLibrary(row interface {
	Scan(...any) error
}) (*model.Library, error) {
	var lib model.Library
	var lastScan, createdAt, updatedAt string
	err := row.Scan(
		&lib.ID,
		&lib.Name,
		&lib.Path,
		&lib.Enabled,
		&lastScan,
		&lib.SongCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	lib.LastScanAt, _ = time.Parse(time.RFC3339, lastScan)
	lib.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	lib.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &lib, nil
}

func (s *sqliteMusicStore) CreateLibrary(ctx context.Context, lib *model.Library) (int64, error) {
	const q = `INSERT INTO libraries (name, path, enabled, last_scan_at, song_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`
	res, err := s.db.ExecContext(ctx, q,
		lib.Name,
		lib.Path,
		lib.Enabled,
		lib.LastScanAt.Format(time.RFC3339),
		lib.SongCount,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *sqliteMusicStore) GetLibrary(ctx context.Context, id int64) (*model.Library, error) {
	const q = `SELECT id, name, path, enabled, last_scan_at, song_count, created_at, updated_at
		FROM libraries WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	lib, err := scanLibrary(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return lib, err
}

func (s *sqliteMusicStore) ListLibraries(ctx context.Context) ([]*model.Library, error) {
	const q = `SELECT id, name, path, enabled, last_scan_at, song_count, created_at, updated_at
		FROM libraries ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var libs []*model.Library
	for rows.Next() {
		lib, err := scanLibrary(rows)
		if err != nil {
			return nil, err
		}
		libs = append(libs, lib)
	}
	return libs, rows.Err()
}

func (s *sqliteMusicStore) UpdateLibrary(ctx context.Context, lib *model.Library) error {
	const q = `UPDATE libraries SET name=?, path=?, enabled=?, last_scan_at=?, song_count=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	_, err := s.db.ExecContext(ctx, q,
		lib.Name,
		lib.Path,
		lib.Enabled,
		lib.LastScanAt.Format(time.RFC3339),
		lib.SongCount,
		lib.ID,
	)
	return err
}

func (s *sqliteMusicStore) DeleteLibrary(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM libraries WHERE id = ?`, id)
	return err
}

// scanArtist reads an artists row into a model.Artist.
func scanArtist(row interface {
	Scan(...any) error
}) (*model.Artist, error) {
	var a model.Artist
	var createdAt, updatedAt string
	err := row.Scan(
		&a.ID,
		&a.Name,
		&a.SortName,
		&a.AlbumCount,
		&a.SongCount,
		&a.CoverArtID,
		&a.Biography,
		&a.MusicBrainzID,
		&a.UserEdited,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &a, nil
}

func (s *sqliteMusicStore) UpsertArtist(ctx context.Context, a *model.Artist) (int64, error) {
	const q = `INSERT INTO artists
		(name, sort_name, album_count, song_count, cover_art_id, biography, musicbrainz_id, user_edited, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			sort_name       = excluded.sort_name,
			album_count     = excluded.album_count,
			song_count      = excluded.song_count,
			cover_art_id    = excluded.cover_art_id,
			biography       = CASE WHEN user_edited=1 AND biography!='' THEN biography ELSE excluded.biography END,
			musicbrainz_id  = CASE WHEN user_edited=1 AND musicbrainz_id!='' THEN musicbrainz_id ELSE excluded.musicbrainz_id END,
			updated_at      = CURRENT_TIMESTAMP`
	res, err := s.db.ExecContext(ctx, q,
		a.Name,
		a.SortName,
		a.AlbumCount,
		a.SongCount,
		a.CoverArtID,
		a.Biography,
		a.MusicBrainzID,
		a.UserEdited,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// ON CONFLICT returns 0 for LastInsertId — fetch the real ID by name
	if id == 0 {
		row := s.db.QueryRowContext(ctx, `SELECT id FROM artists WHERE name = ?`, a.Name)
		err = row.Scan(&id)
	}
	return id, err
}

func (s *sqliteMusicStore) GetArtist(ctx context.Context, id int64) (*model.Artist, error) {
	const q = `SELECT id, name, sort_name, album_count, song_count, cover_art_id, biography,
		musicbrainz_id, user_edited, created_at, updated_at FROM artists WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	a, err := scanArtist(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (s *sqliteMusicStore) GetArtistByName(ctx context.Context, name string) (*model.Artist, error) {
	const q = `SELECT id, name, sort_name, album_count, song_count, cover_art_id, biography,
		musicbrainz_id, user_edited, created_at, updated_at FROM artists WHERE name = ?`
	row := s.db.QueryRowContext(ctx, q, name)
	a, err := scanArtist(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

// artistSelectCols is the standard SELECT column list for artists.
const artistSelectCols = `id, name, sort_name, album_count, song_count, cover_art_id, biography,
	musicbrainz_id, user_edited, created_at, updated_at`

func (s *sqliteMusicStore) ListArtists(ctx context.Context, opts ListOpts) ([]*model.Artist, error) {
	orderDir := "ASC"
	if opts.Desc {
		orderDir = "DESC"
	}
	sortCol := "name"
	if opts.SortBy != "" {
		sortCol = opts.SortBy
	}
	q := fmt.Sprintf(`SELECT %s FROM artists ORDER BY %s %s LIMIT ? OFFSET ?`,
		artistSelectCols, sortCol, orderDir)
	limit := opts.Limit
	if limit <= 0 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, q, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*model.Artist
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (s *sqliteMusicStore) SearchArtists(ctx context.Context, query string, opts ListOpts) ([]*model.Artist, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	const q = `SELECT ` + artistSelectCols + ` FROM artists
		WHERE name LIKE ? ORDER BY name LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, "%"+query+"%", limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artists []*model.Artist
	for rows.Next() {
		a, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		artists = append(artists, a)
	}
	return artists, rows.Err()
}

func (s *sqliteMusicStore) DeleteArtistsWithNoSongs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM artists WHERE song_count = 0`)
	return err
}

// scanAlbum reads an albums row into a model.Album.
func scanAlbum(row interface {
	Scan(...any) error
}) (*model.Album, error) {
	var a model.Album
	var createdAt, updatedAt string
	err := row.Scan(
		&a.ID,
		&a.Title,
		&a.SortTitle,
		&a.ArtistID,
		&a.ArtistName,
		&a.Year,
		&a.Genre,
		&a.SongCount,
		&a.Duration,
		&a.CoverArtID,
		&a.MusicBrainzID,
		&a.UserEdited,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &a, nil
}

// albumSelectCols is the standard SELECT column list for albums.
const albumSelectCols = `id, title, sort_title, artist_id, artist_name, year, genre, song_count, duration,
	cover_art_id, musicbrainz_id, user_edited, created_at, updated_at`

func (s *sqliteMusicStore) UpsertAlbum(ctx context.Context, a *model.Album) (int64, error) {
	const q = `INSERT INTO albums
		(title, sort_title, artist_id, artist_name, year, genre, song_count, duration, cover_art_id, musicbrainz_id, user_edited, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(title, artist_id) DO UPDATE SET
			sort_title     = excluded.sort_title,
			artist_name    = excluded.artist_name,
			song_count     = excluded.song_count,
			duration       = excluded.duration,
			cover_art_id   = excluded.cover_art_id,
			year           = CASE WHEN user_edited=1 AND year!=0 THEN year ELSE excluded.year END,
			genre          = CASE WHEN user_edited=1 AND genre!='' THEN genre ELSE excluded.genre END,
			title          = CASE WHEN user_edited=1 AND title!='' THEN title ELSE excluded.title END,
			musicbrainz_id = CASE WHEN user_edited=1 AND musicbrainz_id!='' THEN musicbrainz_id ELSE excluded.musicbrainz_id END,
			updated_at     = CURRENT_TIMESTAMP`
	res, err := s.db.ExecContext(ctx, q,
		a.Title,
		a.SortTitle,
		a.ArtistID,
		a.ArtistName,
		a.Year,
		a.Genre,
		a.SongCount,
		a.Duration,
		a.CoverArtID,
		a.MusicBrainzID,
		a.UserEdited,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		row := s.db.QueryRowContext(ctx,
			`SELECT id FROM albums WHERE title = ? AND artist_id = ?`, a.Title, a.ArtistID)
		err = row.Scan(&id)
	}
	return id, err
}

func (s *sqliteMusicStore) GetAlbum(ctx context.Context, id int64) (*model.Album, error) {
	q := `SELECT ` + albumSelectCols + ` FROM albums WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	a, err := scanAlbum(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return a, err
}

func (s *sqliteMusicStore) ListAlbums(ctx context.Context, opts ListOpts) ([]*model.Album, error) {
	orderDir := "ASC"
	if opts.Desc {
		orderDir = "DESC"
	}
	sortCol := "title"
	if opts.SortBy != "" {
		sortCol = opts.SortBy
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 500
	}
	q := fmt.Sprintf(`SELECT %s FROM albums ORDER BY %s %s LIMIT ? OFFSET ?`,
		albumSelectCols, sortCol, orderDir)
	rows, err := s.db.QueryContext(ctx, q, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteMusicStore) ListAlbumsByArtist(ctx context.Context, artistID int64) ([]*model.Album, error) {
	q := `SELECT ` + albumSelectCols + ` FROM albums WHERE artist_id = ? ORDER BY year, title`
	rows, err := s.db.QueryContext(ctx, q, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteMusicStore) GetNewestAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	q := fmt.Sprintf(`SELECT %s FROM albums ORDER BY created_at DESC LIMIT ?`, albumSelectCols)
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteMusicStore) GetRandomAlbums(ctx context.Context, limit int) ([]*model.Album, error) {
	q := fmt.Sprintf(`SELECT %s FROM albums ORDER BY RANDOM() LIMIT ?`, albumSelectCols)
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteMusicStore) SearchAlbums(ctx context.Context, query string, opts ListOpts) ([]*model.Album, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT ` + albumSelectCols + ` FROM albums
		WHERE title LIKE ? ORDER BY title LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, "%"+query+"%", limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var albums []*model.Album
	for rows.Next() {
		a, err := scanAlbum(rows)
		if err != nil {
			return nil, err
		}
		albums = append(albums, a)
	}
	return albums, rows.Err()
}

func (s *sqliteMusicStore) DeleteAlbumsWithNoSongs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM albums WHERE song_count = 0`)
	return err
}

// scanSong reads a songs row into a model.Song.
func scanSong(row interface {
	Scan(...any) error
}) (*model.Song, error) {
	var s model.Song
	var lastModified, createdAt, updatedAt string
	err := row.Scan(
		&s.ID,
		&s.LibraryID,
		&s.Path,
		&s.Title,
		&s.SortTitle,
		&s.ArtistID,
		&s.ArtistName,
		&s.AlbumArtistID,
		&s.AlbumArtistName,
		&s.AlbumID,
		&s.AlbumName,
		&s.TrackNumber,
		&s.DiscNumber,
		&s.Year,
		&s.Genre,
		&s.Duration,
		&s.BitRate,
		&s.SampleRate,
		&s.Channels,
		&s.FileSize,
		&s.ContentType,
		&s.FileFormat,
		&s.CoverArtID,
		&s.MBTrackID,
		&s.MBAlbumID,
		&s.MBAlbumArtistID,
		&s.MBArtistID,
		&s.Composer,
		&s.Lyricist,
		&s.Conductor,
		&s.Comment,
		&s.Lyrics,
		&s.BPM,
		&s.ReplayGainTrack,
		&s.ReplayGainAlbum,
		&s.FileHash,
		&lastModified,
		&s.UserEdited,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	s.LastModified, _ = time.Parse(time.RFC3339, lastModified)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &s, nil
}

// songSelectCols is the standard SELECT column list for songs (model fields only).
const songSelectCols = `id, library_id, path, title, sort_title, artist_id, artist_name,
	album_artist_id, album_artist_name, album_id, album_name, track_number, disc_number, year, genre,
	duration, bit_rate, sample_rate, channels, file_size, content_type, file_format, cover_art_id,
	mb_track_id, mb_album_id, mb_album_artist_id, mb_artist_id, composer, lyricist, conductor,
	comment, lyrics, bpm, replay_gain_track, replay_gain_album, file_hash, last_modified,
	user_edited, created_at, updated_at`

func (s *sqliteMusicStore) UpsertSong(ctx context.Context, song *model.Song) (int64, error) {
	const q = `INSERT INTO songs
		(library_id, path, title, sort_title, artist_id, artist_name,
		album_artist_id, album_artist_name, album_id, album_name,
		track_number, disc_number, year, genre, duration, bit_rate, sample_rate, channels,
		file_size, content_type, file_format, cover_art_id,
		mb_track_id, mb_album_id, mb_album_artist_id, mb_artist_id,
		composer, lyricist, conductor, comment, lyrics, bpm,
		replay_gain_track, replay_gain_album, file_hash, last_modified,
		user_edited, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
		ON CONFLICT(path) DO UPDATE SET
			library_id         = excluded.library_id,
			sort_title         = excluded.sort_title,
			artist_id          = excluded.artist_id,
			artist_name        = excluded.artist_name,
			album_artist_id    = excluded.album_artist_id,
			album_artist_name  = excluded.album_artist_name,
			album_id           = excluded.album_id,
			album_name         = excluded.album_name,
			track_number       = excluded.track_number,
			disc_number        = excluded.disc_number,
			year               = excluded.year,
			duration           = excluded.duration,
			bit_rate           = excluded.bit_rate,
			sample_rate        = excluded.sample_rate,
			channels           = excluded.channels,
			file_size          = excluded.file_size,
			content_type       = excluded.content_type,
			file_format        = excluded.file_format,
			cover_art_id       = excluded.cover_art_id,
			file_hash          = excluded.file_hash,
			last_modified      = excluded.last_modified,
			title              = CASE WHEN user_edited=1 AND title!='' THEN title ELSE excluded.title END,
			genre              = CASE WHEN user_edited=1 AND genre!='' THEN genre ELSE excluded.genre END,
			mb_track_id        = CASE WHEN user_edited=1 AND mb_track_id!='' THEN mb_track_id ELSE excluded.mb_track_id END,
			mb_album_id        = CASE WHEN user_edited=1 AND mb_album_id!='' THEN mb_album_id ELSE excluded.mb_album_id END,
			mb_album_artist_id = CASE WHEN user_edited=1 AND mb_album_artist_id!='' THEN mb_album_artist_id ELSE excluded.mb_album_artist_id END,
			mb_artist_id       = CASE WHEN user_edited=1 AND mb_artist_id!='' THEN mb_artist_id ELSE excluded.mb_artist_id END,
			composer           = CASE WHEN user_edited=1 AND composer!='' THEN composer ELSE excluded.composer END,
			comment            = CASE WHEN user_edited=1 AND comment!='' THEN comment ELSE excluded.comment END,
			lyrics             = CASE WHEN user_edited=1 AND lyrics!='' THEN lyrics ELSE excluded.lyrics END,
			lyricist           = excluded.lyricist,
			conductor          = excluded.conductor,
			bpm                = excluded.bpm,
			replay_gain_track  = excluded.replay_gain_track,
			replay_gain_album  = excluded.replay_gain_album,
			updated_at         = CURRENT_TIMESTAMP`

	res, err := s.db.ExecContext(ctx, q,
		song.LibraryID, song.Path, song.Title, song.SortTitle, song.ArtistID, song.ArtistName,
		song.AlbumArtistID, song.AlbumArtistName, song.AlbumID, song.AlbumName,
		song.TrackNumber, song.DiscNumber, song.Year, song.Genre, song.Duration,
		song.BitRate, song.SampleRate, song.Channels, song.FileSize, song.ContentType, song.FileFormat,
		song.CoverArtID, song.MBTrackID, song.MBAlbumID, song.MBAlbumArtistID, song.MBArtistID,
		song.Composer, song.Lyricist, song.Conductor, song.Comment, song.Lyrics, song.BPM,
		song.ReplayGainTrack, song.ReplayGainAlbum, song.FileHash,
		song.LastModified.Format(time.RFC3339),
		song.UserEdited,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		row := s.db.QueryRowContext(ctx, `SELECT id FROM songs WHERE path = ?`, song.Path)
		err = row.Scan(&id)
	}
	return id, err
}

func (s *sqliteMusicStore) GetSong(ctx context.Context, id int64) (*model.Song, error) {
	q := `SELECT ` + songSelectCols + ` FROM songs WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	song, err := scanSong(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return song, err
}

func (s *sqliteMusicStore) GetSongByPath(ctx context.Context, path string) (*model.Song, error) {
	q := `SELECT ` + songSelectCols + ` FROM songs WHERE path = ?`
	row := s.db.QueryRowContext(ctx, q, path)
	song, err := scanSong(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return song, err
}

func (s *sqliteMusicStore) ListSongsByAlbum(ctx context.Context, albumID int64) ([]*model.Song, error) {
	q := `SELECT ` + songSelectCols + ` FROM songs WHERE album_id = ? AND missing = 0
		ORDER BY disc_number, track_number`
	rows, err := s.db.QueryContext(ctx, q, albumID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectSongs(rows)
}

func (s *sqliteMusicStore) ListSongsByArtist(ctx context.Context, artistID int64) ([]*model.Song, error) {
	q := `SELECT ` + songSelectCols + ` FROM songs WHERE artist_id = ? AND missing = 0
		ORDER BY album_name, disc_number, track_number`
	rows, err := s.db.QueryContext(ctx, q, artistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectSongs(rows)
}

func (s *sqliteMusicStore) ListSongsByGenre(ctx context.Context, genre string, opts ListOpts) ([]*model.Song, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 500
	}
	q := `SELECT ` + songSelectCols + ` FROM songs WHERE genre = ? AND missing = 0
		ORDER BY artist_name, album_name, disc_number, track_number LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, genre, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectSongs(rows)
}

func (s *sqliteMusicStore) GetRandomSongs(ctx context.Context, limit int, genre, fromYear, toYear string) ([]*model.Song, error) {
	var whereClauses []string
	var args []interface{}
	whereClauses = append(whereClauses, "missing = 0")

	if genre != "" {
		whereClauses = append(whereClauses, "genre = ?")
		args = append(args, genre)
	}
	if fromYear != "" {
		whereClauses = append(whereClauses, "year >= ?")
		args = append(args, fromYear)
	}
	if toYear != "" {
		whereClauses = append(whereClauses, "year <= ?")
		args = append(args, toYear)
	}

	where := strings.Join(whereClauses, " AND ")
	q := fmt.Sprintf(`SELECT %s FROM songs WHERE %s ORDER BY RANDOM() LIMIT ?`, songSelectCols, where)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectSongs(rows)
}

func (s *sqliteMusicStore) SearchSongs(ctx context.Context, query string, opts ListOpts) ([]*model.Song, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT ` + songSelectCols + ` FROM songs
		WHERE (title LIKE ? OR artist_name LIKE ? OR album_name LIKE ?) AND missing = 0
		ORDER BY title LIMIT ? OFFSET ?`
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, q, like, like, like, limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectSongs(rows)
}

func (s *sqliteMusicStore) MarkSongMissing(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE songs SET missing = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

func (s *sqliteMusicStore) DeleteMissingSongs(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM songs WHERE missing = 1`)
	return err
}

func (s *sqliteMusicStore) IncrementPlayCount(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE songs SET play_count = play_count + 1, last_played_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		id)
	return err
}

func (s *sqliteMusicStore) ListGenres(ctx context.Context) ([]*model.Genre, error) {
	const q = `SELECT genre,
		COUNT(*) AS song_count,
		COUNT(DISTINCT album_id) AS album_count
		FROM songs
		WHERE genre != '' AND missing = 0
		GROUP BY genre
		ORDER BY genre`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []*model.Genre
	for rows.Next() {
		g := &model.Genre{}
		if err := rows.Scan(&g.Name, &g.SongCount, &g.AlbumCount); err != nil {
			return nil, err
		}
		genres = append(genres, g)
	}
	return genres, rows.Err()
}

func (s *sqliteMusicStore) UpsertCoverArt(ctx context.Context, ca *model.CoverArt) (int64, error) {
	const q = `INSERT INTO cover_art (song_id, album_id, data, path, mime_type, width, height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(song_id, album_id) DO UPDATE SET
			data      = excluded.data,
			path      = excluded.path,
			mime_type = excluded.mime_type,
			width     = excluded.width,
			height    = excluded.height`
	res, err := s.db.ExecContext(ctx, q,
		ca.SongID, ca.AlbumID, ca.Data, ca.Path, ca.MimeType, ca.Width, ca.Height)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if id == 0 {
		row := s.db.QueryRowContext(ctx,
			`SELECT id FROM cover_art WHERE song_id = ? AND album_id = ?`, ca.SongID, ca.AlbumID)
		err = row.Scan(&id)
	}
	return id, err
}

func (s *sqliteMusicStore) GetCoverArt(ctx context.Context, id int64) (*model.CoverArt, error) {
	const q = `SELECT id, song_id, album_id, data, path, mime_type, width, height, created_at
		FROM cover_art WHERE id = ?`
	row := s.db.QueryRowContext(ctx, q, id)
	var ca model.CoverArt
	var createdAt string
	err := row.Scan(
		&ca.ID, &ca.SongID, &ca.AlbumID,
		&ca.Data, &ca.Path, &ca.MimeType, &ca.Width, &ca.Height,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ca.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &ca, nil
}

func (s *sqliteMusicStore) CreateScanStatus(ctx context.Context, ss *model.ScanStatus) (int64, error) {
	const q = `INSERT INTO scan_status
		(started_at, finished_at, status, scanned_files, added_files, updated_files, deleted_files, error_count, last_error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	var finishedAt interface{}
	if !ss.FinishedAt.IsZero() {
		finishedAt = ss.FinishedAt.Format(time.RFC3339)
	}
	res, err := s.db.ExecContext(ctx, q,
		ss.StartedAt.Format(time.RFC3339),
		finishedAt,
		ss.Status,
		ss.ScannedFiles,
		ss.AddedFiles,
		ss.UpdatedFiles,
		ss.DeletedFiles,
		ss.ErrorCount,
		ss.LastError,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *sqliteMusicStore) UpdateScanStatus(ctx context.Context, ss *model.ScanStatus) error {
	const q = `UPDATE scan_status SET
		finished_at = ?, status = ?, scanned_files = ?, added_files = ?,
		updated_files = ?, deleted_files = ?, error_count = ?, last_error = ?
		WHERE id = ?`
	var finishedAt interface{}
	if !ss.FinishedAt.IsZero() {
		finishedAt = ss.FinishedAt.Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx, q,
		finishedAt,
		ss.Status,
		ss.ScannedFiles,
		ss.AddedFiles,
		ss.UpdatedFiles,
		ss.DeletedFiles,
		ss.ErrorCount,
		ss.LastError,
		ss.ID,
	)
	return err
}

func (s *sqliteMusicStore) GetLastScanStatus(ctx context.Context) (*model.ScanStatus, error) {
	const q = `SELECT id, started_at, finished_at, status, scanned_files, added_files,
		updated_files, deleted_files, error_count, last_error
		FROM scan_status ORDER BY id DESC LIMIT 1`
	row := s.db.QueryRowContext(ctx, q)
	return scanScanStatus(row)
}

// scanScanStatus reads a scan_status row into a model.ScanStatus.
func scanScanStatus(row interface {
	Scan(...any) error
}) (*model.ScanStatus, error) {
	var ss model.ScanStatus
	var startedAt string
	var finishedAt sql.NullString
	err := row.Scan(
		&ss.ID,
		&startedAt,
		&finishedAt,
		&ss.Status,
		&ss.ScannedFiles,
		&ss.AddedFiles,
		&ss.UpdatedFiles,
		&ss.DeletedFiles,
		&ss.ErrorCount,
		&ss.LastError,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ss.StartedAt, _ = time.Parse(time.RFC3339, startedAt)
	if finishedAt.Valid {
		ss.FinishedAt, _ = time.Parse(time.RFC3339, finishedAt.String)
	}
	return &ss, nil
}

// collectSongs iterates over a *sql.Rows result set and returns all songs.
func collectSongs(rows *sql.Rows) ([]*model.Song, error) {
	var songs []*model.Song
	for rows.Next() {
		song, err := scanSong(rows)
		if err != nil {
			return nil, err
		}
		songs = append(songs, song)
	}
	return songs, rows.Err()
}
