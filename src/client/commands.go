// commands.go - all cassonic-cli command implementations
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// cmdLogin prompts the user for credentials, calls POST /api/v1/auth/login, and saves the token.
func cmdLogin(c *Client, cfg CLIConfig) error {
	reader := bufio.NewReader(os.Stdin)

	defaultURL := cfg.Server.URL
	if defaultURL == "" {
		defaultURL = "http://localhost:4533"
	}
	fmt.Printf("Server URL [%s]: ", defaultURL)
	serverInput, _ := reader.ReadString('\n')
	serverInput = strings.TrimRight(serverInput, "\r\n")
	if serverInput == "" {
		serverInput = defaultURL
	}
	c.baseURL = strings.TrimRight(serverInput, "/")

	fmt.Print("Username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimRight(username, "\r\n")

	password, err := readPassword("Password: ")
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}

	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Message string `json:"message"`
	}
	payload := map[string]string{
		"username": username,
		"password": password,
	}
	if err := c.post("/api/v1/auth/login", payload, &result); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if result.Data.Token == "" {
		return fmt.Errorf("server did not return a token")
	}

	// Persist token and update config with new server URL.
	if err := saveToken(result.Data.Token); err != nil {
		return fmt.Errorf("saving token: %w", err)
	}
	cfg.Server.URL = c.baseURL
	cfg.Token = result.Data.Token
	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	printSuccess("Logged in successfully. Token saved.")
	return nil
}

// cmdLogout calls DELETE /api/v1/auth/logout and removes the cached token.
func cmdLogout(c *Client) error {
	if err := c.delete("/api/v1/auth/logout"); err != nil {
		// Best-effort: still delete local token even if server returns an error.
		deleteToken()
		return fmt.Errorf("logout request failed (token still deleted locally): %w", err)
	}
	deleteToken()
	printSuccess("Logged out.")
	return nil
}

// cmdStatus calls GET /api/v1/health and prints server info.
func cmdStatus(c *Client, wantJSON bool) error {
	data, code, err := c.getRaw("/api/v1/health")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Status  string `json:"status"`
			Version string `json:"version"`
			Uptime  string `json:"uptime"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal(data, &result); jsonErr != nil || code >= 400 {
		fmt.Printf("Server: %s  HTTP %d\n", c.baseURL, code)
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "SERVER STATUS"))
	fmt.Fprintf(tw, "URL:\t%s\n", c.baseURL)
	fmt.Fprintf(tw, "Status:\t%s\n", result.Data.Status)
	fmt.Fprintf(tw, "Version:\t%s\n", result.Data.Version)
	fmt.Fprintf(tw, "Uptime:\t%s\n", result.Data.Uptime)
	tw.Flush()
	return nil
}

// cmdScan triggers a library scan via POST /api/v1/libraries/{id}/scan.
func cmdScan(c *Client, libraryID string, full bool, wantJSON bool) error {
	if libraryID == "" {
		libraryID = "default"
	}
	payload := map[string]bool{"full": full}
	data, _, err := c.postRaw("/api/v1/libraries/"+libraryID+"/scan", payload)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	printSuccess("Library scan triggered.")
	return nil
}

// cmdScanStatus prints the current scan status from GET /api/v1/scan/status.
func cmdScanStatus(c *Client, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/scan/status")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Scanning bool   `json:"scanning"`
			Progress int    `json:"progress"`
			Total    int    `json:"total"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "SCAN STATUS"))
	fmt.Fprintf(tw, "Scanning:\t%v\n", result.Data.Scanning)
	fmt.Fprintf(tw, "Progress:\t%d / %d\n", result.Data.Progress, result.Data.Total)
	fmt.Fprintf(tw, "Status:\t%s\n", result.Data.Status)
	tw.Flush()
	return nil
}

// cmdArtists lists artists from GET /api/v1/artists.
func cmdArtists(c *Client, page, limit int, wantJSON bool) error {
	path := fmt.Sprintf("/api/v1/artists?page=%d&limit=%d", page, limit)
	data, _, err := c.getRaw(path)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tNAME"))
	for _, a := range result.Data {
		fmt.Fprintf(tw, "%s\t%s\n", a.ID, a.Name)
	}
	tw.Flush()
	return nil
}

// cmdAlbums lists albums from GET /api/v1/albums.
func cmdAlbums(c *Client, page, limit int, wantJSON bool) error {
	path := fmt.Sprintf("/api/v1/albums?page=%d&limit=%d", page, limit)
	data, _, err := c.getRaw(path)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Year   int    `json:"year"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tTITLE\tARTIST\tYEAR"))
	for _, a := range result.Data {
		year := ""
		if a.Year > 0 {
			year = strconv.Itoa(a.Year)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", a.ID, a.Title, a.Artist, year)
	}
	tw.Flush()
	return nil
}

// cmdSongs lists songs from GET /api/v1/songs.
func cmdSongs(c *Client, page, limit int, wantJSON bool) error {
	path := fmt.Sprintf("/api/v1/songs?page=%d&limit=%d", page, limit)
	data, _, err := c.getRaw(path)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID     string `json:"id"`
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Album  string `json:"album"`
			Track  int    `json:"track"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tTRACK\tTITLE\tARTIST\tALBUM"))
	for _, s := range result.Data {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n", s.ID, s.Track, s.Title, s.Artist, s.Album)
	}
	tw.Flush()
	return nil
}

// cmdGenres lists genres from GET /api/v1/genres.
func cmdGenres(c *Client, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/genres")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tNAME"))
	for _, g := range result.Data {
		fmt.Fprintf(tw, "%s\t%s\n", g.ID, g.Name)
	}
	tw.Flush()
	return nil
}

// cmdSearch searches all content via GET /api/v1/search.
func cmdSearch(c *Client, query string, wantJSON bool) error {
	path := "/api/v1/search?q=" + urlQueryEscape(query)
	data, _, err := c.getRaw(path)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Artists []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"artists"`
			Albums []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Artist string `json:"artist"`
			} `json:"albums"`
			Songs []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Artist string `json:"artist"`
			} `json:"songs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	if len(result.Data.Artists) > 0 {
		fmt.Fprintln(tw, colorize(ansiBoldCyan, "ARTISTS"))
		fmt.Fprintln(tw, colorize(ansiBold, "ID\tNAME"))
		for _, a := range result.Data.Artists {
			fmt.Fprintf(tw, "%s\t%s\n", a.ID, a.Name)
		}
		fmt.Fprintln(tw)
	}
	if len(result.Data.Albums) > 0 {
		fmt.Fprintln(tw, colorize(ansiBoldCyan, "ALBUMS"))
		fmt.Fprintln(tw, colorize(ansiBold, "ID\tTITLE\tARTIST"))
		for _, a := range result.Data.Albums {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", a.ID, a.Title, a.Artist)
		}
		fmt.Fprintln(tw)
	}
	if len(result.Data.Songs) > 0 {
		fmt.Fprintln(tw, colorize(ansiBoldCyan, "SONGS"))
		fmt.Fprintln(tw, colorize(ansiBold, "ID\tTITLE\tARTIST"))
		for _, s := range result.Data.Songs {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", s.ID, s.Title, s.Artist)
		}
	}
	tw.Flush()
	return nil
}

// cmdPlaylists lists playlists from GET /api/v1/playlists.
func cmdPlaylists(c *Client, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/playlists")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			SongCount int    `json:"song_count"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tNAME\tSONGS"))
	for _, p := range result.Data {
		fmt.Fprintf(tw, "%s\t%s\t%d\n", p.ID, p.Name, p.SongCount)
	}
	tw.Flush()
	return nil
}

// cmdPlaylist shows detail for a single playlist via GET /api/v1/playlists/{id}.
func cmdPlaylist(c *Client, id string, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/playlists/" + id)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Songs []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Artist string `json:"artist"`
				Album  string `json:"album"`
			} `json:"songs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintf(tw, "%s %s\n", colorize(ansiBoldCyan, "Playlist:"), result.Data.Name)
	fmt.Fprintf(tw, "ID:\t%s\n", result.Data.ID)
	fmt.Fprintln(tw)
	fmt.Fprintln(tw, colorize(ansiBold, "SONG ID\tTITLE\tARTIST\tALBUM"))
	for _, s := range result.Data.Songs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, s.Title, s.Artist, s.Album)
	}
	tw.Flush()
	return nil
}

// cmdPlaylistCreate creates a new playlist via POST /api/v1/playlists.
func cmdPlaylistCreate(c *Client, name string, wantJSON bool) error {
	payload := map[string]string{"name": name}
	data, _, err := c.postRaw("/api/v1/playlists", payload)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	printSuccess(fmt.Sprintf("Playlist created: %s (id: %s)", result.Data.Name, result.Data.ID))
	return nil
}

// cmdPlaylistAdd adds a song to a playlist via POST /api/v1/playlists/{id}/songs.
func cmdPlaylistAdd(c *Client, playlistID, songID string) error {
	payload := map[string]string{"song_id": songID}
	if err := c.post("/api/v1/playlists/"+playlistID+"/songs", payload, nil); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Song %s added to playlist %s.", songID, playlistID))
	return nil
}

// cmdPlaylistRemove removes a song from a playlist via DELETE /api/v1/playlists/{id}/songs/{songID}.
func cmdPlaylistRemove(c *Client, playlistID, songID string) error {
	if err := c.delete("/api/v1/playlists/" + playlistID + "/songs/" + songID); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Song %s removed from playlist %s.", songID, playlistID))
	return nil
}

// cmdTags retrieves tags for a song via GET /api/v1/songs/{id}/tags.
func cmdTags(c *Client, songID string, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/songs/" + songID + "/tags")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			Title  string `json:"title"`
			Artist string `json:"artist"`
			Album  string `json:"album"`
			Year   int    `json:"year"`
			Track  int    `json:"track"`
			Genre  string `json:"genre"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "SONG TAGS"))
	fmt.Fprintf(tw, "Title:\t%s\n", result.Data.Title)
	fmt.Fprintf(tw, "Artist:\t%s\n", result.Data.Artist)
	fmt.Fprintf(tw, "Album:\t%s\n", result.Data.Album)
	fmt.Fprintf(tw, "Year:\t%d\n", result.Data.Year)
	fmt.Fprintf(tw, "Track:\t%d\n", result.Data.Track)
	fmt.Fprintf(tw, "Genre:\t%s\n", result.Data.Genre)
	tw.Flush()
	return nil
}

// TagSetFields carries the optional tag fields for cmdTagsSet.
type TagSetFields struct {
	Title  string
	Artist string
	Album  string
	Year   string
	Track  string
	Genre  string
}

// cmdTagsSet updates one or more tag fields on a song via PUT /api/v1/songs/{id}/tags.
func cmdTagsSet(c *Client, songID string, fields TagSetFields, wantJSON bool) error {
	payload := map[string]any{}
	if fields.Title != "" {
		payload["title"] = fields.Title
	}
	if fields.Artist != "" {
		payload["artist"] = fields.Artist
	}
	if fields.Album != "" {
		payload["album"] = fields.Album
	}
	if fields.Year != "" {
		year, err := strconv.Atoi(fields.Year)
		if err != nil {
			return fmt.Errorf("invalid year: %s", fields.Year)
		}
		payload["year"] = year
	}
	if fields.Track != "" {
		track, err := strconv.Atoi(fields.Track)
		if err != nil {
			return fmt.Errorf("invalid track number: %s", fields.Track)
		}
		payload["track"] = track
	}
	if fields.Genre != "" {
		payload["genre"] = fields.Genre
	}
	if len(payload) == 0 {
		return fmt.Errorf("no tag fields specified; use --title, --artist, --album, --year, --track, or --genre")
	}
	var out any
	if err := c.put("/api/v1/songs/"+songID+"/tags", payload, &out); err != nil {
		return err
	}
	if wantJSON {
		if data, err := json.MarshalIndent(out, "", "  "); err == nil {
			fmt.Println(string(data))
		}
		return nil
	}
	printSuccess(fmt.Sprintf("Tags updated for song %s.", songID))
	return nil
}

// cmdIcecastList lists Icecast mounts via GET /api/v1/icecast/mounts.
func cmdIcecastList(c *Client, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/icecast/mounts")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID     string `json:"id"`
			Mount  string `json:"mount"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tMOUNT\tSTATUS"))
	for _, m := range result.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", m.ID, m.Mount, m.Status)
	}
	tw.Flush()
	return nil
}

// cmdIcecastStart starts an Icecast mount via POST /api/v1/icecast/mounts/{id}/start.
func cmdIcecastStart(c *Client, mountID string) error {
	if err := c.post("/api/v1/icecast/mounts/"+mountID+"/start", nil, nil); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Icecast mount %s started.", mountID))
	return nil
}

// cmdIcecastStop stops an Icecast mount via POST /api/v1/icecast/mounts/{id}/stop.
func cmdIcecastStop(c *Client, mountID string) error {
	if err := c.post("/api/v1/icecast/mounts/"+mountID+"/stop", nil, nil); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("Icecast mount %s stopped.", mountID))
	return nil
}

// cmdUsersList lists all users via GET /api/v1/admin/users.
func cmdUsersList(c *Client, wantJSON bool) error {
	data, _, err := c.getRaw("/api/v1/admin/users")
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data []struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Role     string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	tw := newTabWriter()
	fmt.Fprintln(tw, colorize(ansiBoldCyan, "ID\tUSERNAME\tEMAIL\tROLE"))
	for _, u := range result.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", u.ID, u.Username, u.Email, u.Role)
	}
	tw.Flush()
	return nil
}

// cmdUsersCreate creates a user via POST /api/v1/admin/users.
// Prompts for a password; admin flag makes the user an admin.
func cmdUsersCreate(c *Client, username string, admin bool, wantJSON bool) error {
	password, err := readPassword("Password for " + username + ": ")
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	role := "user"
	if admin {
		role = "admin"
	}
	payload := map[string]any{
		"username": username,
		"password": password,
		"role":     role,
	}
	data, _, err := c.postRaw("/api/v1/admin/users", payload)
	if err != nil {
		return err
	}
	if wantJSON {
		fmt.Println(string(data))
		return nil
	}
	var result struct {
		OK   bool `json:"ok"`
		Data struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		fmt.Println(string(data))
		return nil
	}
	printSuccess(fmt.Sprintf("User created: %s (id: %s)", result.Data.Username, result.Data.ID))
	return nil
}

// cmdUsersDelete deletes a user via DELETE /api/v1/admin/users/{username}.
func cmdUsersDelete(c *Client, username string) error {
	if err := c.delete("/api/v1/admin/users/" + username); err != nil {
		return err
	}
	printSuccess(fmt.Sprintf("User %s deleted.", username))
	return nil
}

// urlQueryEscape performs minimal URL query escaping for search terms.
// Uses only stdlib — no net/url import needed beyond what we already have.
func urlQueryEscape(s string) string {
	// Simple replacement of common special characters.
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == ' ':
			b.WriteString("+")
		case (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') ||
			ch == '-' || ch == '_' || ch == '.' || ch == '~':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
