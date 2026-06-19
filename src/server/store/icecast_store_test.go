package store

import (
	"context"
	"testing"

	"github.com/local/cassonic/src/server/model"
)

// newTestIcecastStore creates an in-memory SQLite DB and returns an IcecastStore.
func newTestIcecastStore(t *testing.T) IcecastStore {
	t.Helper()
	db, err := openDB(":memory:", serverSchema)
	if err != nil {
		t.Fatalf("openDB :memory:: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return &sqliteIcecastStore{db: db}
}

// sampleIcecastServer returns a minimal IcecastServer for testing.
func sampleIcecastServer(name string) *model.IcecastServer {
	return &model.IcecastServer{
		Name:       name,
		Host:       "icecast.example.com",
		Port:       8000,
		Protocol:   "http",
		SourceUser: "source",
		SourcePass: "hackme",
		Enabled:    true,
	}
}

// sampleIcecastMount returns a minimal IcecastMount for a given server.
func sampleIcecastMount(serverID int64, path string) *model.IcecastMount {
	return &model.IcecastMount{
		ServerID:  serverID,
		MountPath: path,
		Name:      "Test Stream",
		Enabled:   true,
		Status:    model.StatusDisconnected,
	}
}

// TestCreateGetIcecastServerRoundtrip verifies Create+Get for servers.
func TestCreateGetIcecastServerRoundtrip(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srv := sampleIcecastServer("Main Server")
	id, err := s.CreateServer(ctx, srv)
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateServer returned non-positive ID: %d", id)
	}

	got, err := s.GetServer(ctx, id)
	if err != nil {
		t.Fatalf("GetServer: %v", err)
	}
	if got == nil {
		t.Fatal("GetServer returned nil for just-created server")
	}
	if got.Name != "Main Server" {
		t.Errorf("Name: got %q, want 'Main Server'", got.Name)
	}
	if got.Host != "icecast.example.com" {
		t.Errorf("Host: got %q, want icecast.example.com", got.Host)
	}
}

// TestGetIcecastServerMissingReturnsNil verifies GetServer for unknown ID returns (nil, nil).
func TestGetIcecastServerMissingReturnsNil(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	got, err := s.GetServer(ctx, 99999)
	if err != nil {
		t.Fatalf("GetServer (missing): %v", err)
	}
	if got != nil {
		t.Error("GetServer (missing): expected nil, got non-nil")
	}
}

// TestListIcecastServersEmpty verifies ListServers on an empty store returns empty slice.
func TestListIcecastServersEmpty(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	servers, err := s.ListServers(ctx)
	if err != nil {
		t.Fatalf("ListServers (empty): %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("ListServers (empty): got %d, want 0", len(servers))
	}
}

// TestListIcecastServersMultiple verifies ListServers returns all servers.
func TestListIcecastServersMultiple(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	for _, name := range []string{"Server A", "Server B", "Server C"} {
		if _, err := s.CreateServer(ctx, sampleIcecastServer(name)); err != nil {
			t.Fatalf("CreateServer(%q): %v", name, err)
		}
	}

	servers, err := s.ListServers(ctx)
	if err != nil {
		t.Fatalf("ListServers: %v", err)
	}
	if len(servers) != 3 {
		t.Errorf("ListServers: got %d, want 3", len(servers))
	}
}

// TestUpdateIcecastServer verifies UpdateServer persists changed fields.
func TestUpdateIcecastServer(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srv := sampleIcecastServer("Old Name")
	id, err := s.CreateServer(ctx, srv)
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	srv.ID = id
	srv.Name = "New Name"
	srv.Port = 8080
	srv.Enabled = false
	if err := s.UpdateServer(ctx, srv); err != nil {
		t.Fatalf("UpdateServer: %v", err)
	}

	got, err := s.GetServer(ctx, id)
	if err != nil {
		t.Fatalf("GetServer after update: %v", err)
	}
	if got.Name != "New Name" {
		t.Errorf("Name after update: got %q, want 'New Name'", got.Name)
	}
	if got.Port != 8080 {
		t.Errorf("Port after update: got %d, want 8080", got.Port)
	}
	if got.Enabled {
		t.Error("Enabled after update: got true, want false")
	}
}

// TestDeleteIcecastServerRemovesRow verifies DeleteServer removes the server.
func TestDeleteIcecastServerRemovesRow(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	id, err := s.CreateServer(ctx, sampleIcecastServer("To Delete"))
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	if err := s.DeleteServer(ctx, id); err != nil {
		t.Fatalf("DeleteServer: %v", err)
	}

	got, err := s.GetServer(ctx, id)
	if err != nil {
		t.Fatalf("GetServer after delete: %v", err)
	}
	if got != nil {
		t.Error("GetServer after delete: expected nil, got non-nil")
	}
}

// TestCreateGetIcecastMountRoundtrip verifies Create+Get for mounts.
func TestCreateGetIcecastMountRoundtrip(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srvID, err := s.CreateServer(ctx, sampleIcecastServer("Srv"))
	if err != nil {
		t.Fatalf("CreateServer: %v", err)
	}

	m := sampleIcecastMount(srvID, "/live")
	id, err := s.CreateMount(ctx, m)
	if err != nil {
		t.Fatalf("CreateMount: %v", err)
	}
	if id <= 0 {
		t.Fatalf("CreateMount returned non-positive ID: %d", id)
	}

	got, err := s.GetMount(ctx, id)
	if err != nil {
		t.Fatalf("GetMount: %v", err)
	}
	if got == nil {
		t.Fatal("GetMount returned nil for just-created mount")
	}
	if got.MountPath != "/live" {
		t.Errorf("MountPath: got %q, want /live", got.MountPath)
	}
	if got.ServerID != srvID {
		t.Errorf("ServerID: got %d, want %d", got.ServerID, srvID)
	}
}

// TestGetIcecastMountMissingReturnsNil verifies GetMount for unknown ID returns (nil, nil).
func TestGetIcecastMountMissingReturnsNil(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	got, err := s.GetMount(ctx, 99999)
	if err != nil {
		t.Fatalf("GetMount (missing): %v", err)
	}
	if got != nil {
		t.Error("GetMount (missing): expected nil, got non-nil")
	}
}

// TestListMountsByServer verifies ListMountsByServer filters by server.
func TestListMountsByServer(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srv1ID, _ := s.CreateServer(ctx, sampleIcecastServer("Srv1"))
	srv2ID, _ := s.CreateServer(ctx, sampleIcecastServer("Srv2"))

	for _, path := range []string{"/live", "/stream", "/relay"} {
		if _, err := s.CreateMount(ctx, sampleIcecastMount(srv1ID, path)); err != nil {
			t.Fatalf("CreateMount(srv1, %q): %v", path, err)
		}
	}
	if _, err := s.CreateMount(ctx, sampleIcecastMount(srv2ID, "/other")); err != nil {
		t.Fatalf("CreateMount(srv2): %v", err)
	}

	mounts1, err := s.ListMountsByServer(ctx, srv1ID)
	if err != nil {
		t.Fatalf("ListMountsByServer(srv1): %v", err)
	}
	if len(mounts1) != 3 {
		t.Errorf("ListMountsByServer(srv1): got %d, want 3", len(mounts1))
	}

	mounts2, err := s.ListMountsByServer(ctx, srv2ID)
	if err != nil {
		t.Fatalf("ListMountsByServer(srv2): %v", err)
	}
	if len(mounts2) != 1 {
		t.Errorf("ListMountsByServer(srv2): got %d, want 1", len(mounts2))
	}
}

// TestUpdateMountStatus verifies UpdateMountStatus changes the status and current song.
func TestUpdateMountStatus(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srvID, _ := s.CreateServer(ctx, sampleIcecastServer("Srv"))
	id, err := s.CreateMount(ctx, sampleIcecastMount(srvID, "/live"))
	if err != nil {
		t.Fatalf("CreateMount: %v", err)
	}

	if err := s.UpdateMountStatus(ctx, id, model.StatusConnected, "Artist - Song", ""); err != nil {
		t.Fatalf("UpdateMountStatus: %v", err)
	}

	got, err := s.GetMount(ctx, id)
	if err != nil {
		t.Fatalf("GetMount after UpdateMountStatus: %v", err)
	}
	if got.Status != model.StatusConnected {
		t.Errorf("Status: got %q, want Connected", got.Status)
	}
}

// TestDeleteIcecastMountRemovesRow verifies DeleteMount removes the mount.
func TestDeleteIcecastMountRemovesRow(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srvID, _ := s.CreateServer(ctx, sampleIcecastServer("Srv"))
	id, err := s.CreateMount(ctx, sampleIcecastMount(srvID, "/tmp"))
	if err != nil {
		t.Fatalf("CreateMount: %v", err)
	}

	if err := s.DeleteMount(ctx, id); err != nil {
		t.Fatalf("DeleteMount: %v", err)
	}

	got, err := s.GetMount(ctx, id)
	if err != nil {
		t.Fatalf("GetMount after delete: %v", err)
	}
	if got != nil {
		t.Error("GetMount after delete: expected nil, got non-nil")
	}
}

// TestListMountsGlobal verifies ListMounts returns all mounts across servers.
func TestListMountsGlobal(t *testing.T) {
	s := newTestIcecastStore(t)
	ctx := context.Background()

	srv1ID, _ := s.CreateServer(ctx, sampleIcecastServer("S1"))
	srv2ID, _ := s.CreateServer(ctx, sampleIcecastServer("S2"))

	s.CreateMount(ctx, sampleIcecastMount(srv1ID, "/a"))
	s.CreateMount(ctx, sampleIcecastMount(srv2ID, "/b"))

	mounts, err := s.ListMounts(ctx)
	if err != nil {
		t.Fatalf("ListMounts: %v", err)
	}
	if len(mounts) != 2 {
		t.Errorf("ListMounts: got %d, want 2", len(mounts))
	}
}
