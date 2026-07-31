package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	mw "github.com/local/cassonic/src/server/middleware"
	"github.com/local/cassonic/src/server/model"
	"github.com/local/cassonic/src/server/store"
)

// configUserStore is a configurable stub implementing store.UserStore for handler tests.
type configUserStore struct {
	// user CRUD
	createUserID            int64
	createUserErr           error
	getUserResult           *model.User
	getUserErr              error
	getUserByUsernameResult *model.User
	getUserByUsernameErr    error
	getUserByEmailResult    *model.User
	getUserByEmailErr       error
	updateUserErr           error
	deleteUserErr           error
	listUsersResult         []*model.User
	listUsersErr            error

	// login tracking
	incrementLoginAttemptsErr error
	resetLoginAttemptsErr     error
	setLockedUntilErr         error
	updateLastLoginErr        error

	// API tokens
	createAPITokenErr         error
	getAPITokenByHashResult   *model.APIToken
	getAPITokenByHashErr      error
	listAPITokensResult       []*model.APIToken
	listAPITokensErr          error
	deleteAPITokenErr         error
	updateAPITokenLastUsedErr error

	// sessions
	createSessionErr        error
	getSessionByHashResult  *store.Session
	getSessionByHashErr     error
	deleteSessionErr        error
	deleteUserSessionsErr   error
	purgeExpiredSessionsErr error

	// subsonic
	getSubsonicPasswordEncrypted string
	getSubsonicPasswordOK        bool
	getSubsonicPasswordErr       error
	setSubsonicPasswordErr       error

	// radio stations
	createRadioStationID    int64
	createRadioStationErr   error
	getRadioStationResult   *model.InternetRadioStation
	getRadioStationErr      error
	listRadioStationsResult []*model.InternetRadioStation
	listRadioStationsErr    error
	updateRadioStationErr   error
	deleteRadioStationErr   error
}

func (s *configUserStore) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	return s.createUserID, s.createUserErr
}

func (s *configUserStore) GetUser(ctx context.Context, id int64) (*model.User, error) {
	return s.getUserResult, s.getUserErr
}

func (s *configUserStore) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.getUserByUsernameResult, s.getUserByUsernameErr
}

func (s *configUserStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return s.getUserByEmailResult, s.getUserByEmailErr
}

func (s *configUserStore) UpdateUser(ctx context.Context, u *model.User) error {
	return s.updateUserErr
}

func (s *configUserStore) DeleteUser(ctx context.Context, id int64) error {
	return s.deleteUserErr
}

func (s *configUserStore) ListUsers(ctx context.Context) ([]*model.User, error) {
	return s.listUsersResult, s.listUsersErr
}

func (s *configUserStore) IncrementLoginAttempts(ctx context.Context, id int64) error {
	return s.incrementLoginAttemptsErr
}

func (s *configUserStore) ResetLoginAttempts(ctx context.Context, id int64) error {
	return s.resetLoginAttemptsErr
}

func (s *configUserStore) SetLockedUntil(ctx context.Context, id int64, until time.Time) error {
	return s.setLockedUntilErr
}

func (s *configUserStore) UpdateLastLogin(ctx context.Context, id int64) error {
	return s.updateLastLoginErr
}

func (s *configUserStore) CreateAPIToken(ctx context.Context, t *model.APIToken) error {
	return s.createAPITokenErr
}

func (s *configUserStore) GetAPITokenByHash(ctx context.Context, hash string) (*model.APIToken, error) {
	return s.getAPITokenByHashResult, s.getAPITokenByHashErr
}

func (s *configUserStore) ListAPITokens(ctx context.Context, userID int64) ([]*model.APIToken, error) {
	return s.listAPITokensResult, s.listAPITokensErr
}

func (s *configUserStore) DeleteAPIToken(ctx context.Context, id int64) error {
	return s.deleteAPITokenErr
}

func (s *configUserStore) UpdateAPITokenLastUsed(ctx context.Context, id int64) error {
	return s.updateAPITokenLastUsedErr
}

func (s *configUserStore) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, clientName string) error {
	return s.createSessionErr
}

func (s *configUserStore) GetSessionByHash(ctx context.Context, tokenHash string) (*store.Session, error) {
	return s.getSessionByHashResult, s.getSessionByHashErr
}

func (s *configUserStore) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.deleteSessionErr
}

func (s *configUserStore) DeleteUserSessions(ctx context.Context, userID int64) error {
	return s.deleteUserSessionsErr
}

func (s *configUserStore) PurgeExpiredSessions(ctx context.Context) error {
	return s.purgeExpiredSessionsErr
}

func (s *configUserStore) GetSubsonicPassword(ctx context.Context, username string) (string, bool, error) {
	return s.getSubsonicPasswordEncrypted, s.getSubsonicPasswordOK, s.getSubsonicPasswordErr
}

func (s *configUserStore) SetSubsonicPassword(ctx context.Context, username string, encrypted string) error {
	return s.setSubsonicPasswordErr
}

func (s *configUserStore) CreateRadioStation(ctx context.Context, st *model.InternetRadioStation) (int64, error) {
	return s.createRadioStationID, s.createRadioStationErr
}

func (s *configUserStore) GetRadioStation(ctx context.Context, id int64) (*model.InternetRadioStation, error) {
	return s.getRadioStationResult, s.getRadioStationErr
}

func (s *configUserStore) ListRadioStations(ctx context.Context) ([]*model.InternetRadioStation, error) {
	return s.listRadioStationsResult, s.listRadioStationsErr
}

func (s *configUserStore) UpdateRadioStation(ctx context.Context, st *model.InternetRadioStation) error {
	return s.updateRadioStationErr
}

func (s *configUserStore) DeleteRadioStation(ctx context.Context, id int64) error {
	return s.deleteRadioStationErr
}

// errUserStore is a sentinel error used in user store tests.
var errUserStore = errors.New("user store error")

// mustHashPassword computes an Argon2id hash or panics; used in test setup.
func mustHashPassword(password string) string {
	h, err := hashPassword(password)
	if err != nil {
		panic(err)
	}
	return h
}

// newUserHandler returns a Handler backed by the given configUserStore with a stub music store.
func newUserHandler(us *configUserStore) *Handler {
	return newHealthHandler(&store.DB{
		Music: &stubMusicStore{},
		Users: us,
	})
}

// newUserHandlerWithAuth returns a Handler backed by the given configUserStore.
// The isAdmin flag is unused here; auth injection is done via withAuthUser per-request.
func newUserHandlerWithAuth(us *configUserStore, isAdmin bool) *Handler {
	return newUserHandler(us)
}

// withAuthUser injects an authenticated user into the request context.
func withAuthUser(r *http.Request, id int64, username string, isAdmin bool) *http.Request {
	return r.WithContext(mw.WithUser(r.Context(), &mw.AuthUser{ID: id, Username: username, IsAdmin: isAdmin}))
}
