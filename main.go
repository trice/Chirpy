package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/trice/Chirpy/internal/auth"
	"github.com/trice/Chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
    queries *database.Queries
    platform string
    tokenSecret string
}

func (cfg *apiConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cfg.fileserverHits.Add(1)
        next.ServeHTTP(w, r)
    })
}

func (cfg * apiConfig) GetHits(writer http.ResponseWriter, request *http.Request) {
    htmlFormat := `<html>
                      <body>
                        <h1>Welcome, Chirpy Admin</h1>
                        <p>Chirpy has been visited %d times!</p>
                      </body>
                    </html>`
    writer.Header().Set("Content-Type", "text/html; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    hits := cfg.fileserverHits.Load()
    message := fmt.Sprintf(htmlFormat, hits)
    writer.Write([]byte(message))
}

func (cfg * apiConfig) createUser(writer http.ResponseWriter, request *http.Request) {
    // decode the body to a struct and then check the length of the string
    type body struct {
        Password string `json:"password"`
        Email string `json:"email"`
    }
    data, err := io.ReadAll(request.Body)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    hashPass, err := auth.HashPassword(rb.Password)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    dbUserParams := database.CreateUserParams {
        Email: rb.Email,
        HashedPassword: hashPass,
    }

    user, err := cfg.queries.CreateUser(request.Context(), dbUserParams)
    if err != nil {
        writer.Header().Set("Content-Type", "application/json; charset=utf-8")
        writer.WriteHeader(http.StatusBadRequest)
        writer.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    d, _ := json.Marshal(user)
    writer.Header().Set("Content-Type", "application/json; charset=utf-8")
    writer.WriteHeader(http.StatusCreated)
    writer.Write(d)
}

func (cfg * apiConfig) updateUser(w http.ResponseWriter, r *http.Request) {
    validUuid := validateAccessToken(r, w, cfg)
    if validUuid == (uuid.UUID{}) {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    type body struct {
        Password string `json:"password"`
        Email string `json:"email"`
    }
    data, err := io.ReadAll(r.Body)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    hashPass, err := auth.HashPassword(rb.Password)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    updateParams := database.UpdateUserParams {
        Email: rb.Email,
        HashedPassword: hashPass,
        ID: uuid.NullUUID{ UUID: validUuid, Valid: true, },
    }
    updateUser, err := cfg.queries.UpdateUser(r.Context(), updateParams)
    if err != nil {

    }

    d, _ := json.Marshal(updateUser)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg * apiConfig) resetHits(writer http.ResponseWriter, request *http.Request) {
    cfg.fileserverHits.Store(0)
    if cfg.platform == "dev" {
        cfg.queries.DeleteUser(request.Context())
    } else {
        writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
        writer.WriteHeader(http.StatusForbidden)
    }
}

func (cfg * apiConfig) createChirp(w http.ResponseWriter, r *http.Request) {
    // decode the body to a struct and then check the length of the string
    type body struct {
        Body string `json:"body"`
        UserId uuid.UUID `json:"user_id"`
    }

    validUuid := validateAccessToken(r, w, cfg)
    if validUuid == (uuid.UUID{}) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
        return
    }

    data, err := io.ReadAll(r.Body)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb.Body = scrubMessage(rb.Body)

    if len(rb.Body) > 140 {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error": "Chirp is too long"}`))
        return
    }

    newChirp := database.CreateChirpParams {
        Body: rb.Body,
        UserID: validUuid,
    }

    dbResult, err := cfg.queries.CreateChirp(r.Context(), newChirp)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    d, _ := json.Marshal(dbResult)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusCreated)
    w.Write(d)
}

// check of the Access Token is valide and if so return the UUID of the user.
func validateAccessToken(r *http.Request, w http.ResponseWriter, cfg *apiConfig) (uuid.UUID) {
	jwt, err := auth.GetBearerToken(r.Header)
	if err != nil {
		return uuid.UUID{}
	}

	validUuid, err := auth.ValidateJWT(jwt, cfg.tokenSecret)
	if err != nil {
		return uuid.UUID{}
	}

	return validUuid
}

func (cfg *apiConfig) getChirps(w http.ResponseWriter, r *http.Request)  {
    chirpsAscByCreate, err := cfg.queries.GetChirps(r.Context())
    if err != nil {
        http.Error(w, "Error reading chirps", http.StatusNotFound)
    }

    d, _ := json.Marshal(chirpsAscByCreate)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg* apiConfig) getChirpBy(w http.ResponseWriter, r *http.Request)  {
    var chirpId uuid.NullUUID
    chirpId.UUID = uuid.MustParse(r.PathValue("chirpID"))
    chirpId.Valid = true
    chirpResult, err := cfg.queries.GetChirpById(r.Context(), chirpId)
    if err != nil {
        http.Error(w, "chirp not found", http.StatusNotFound)
        return
    }

    d, _ := json.Marshal(chirpResult)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg *apiConfig) deleteChirp(w http.ResponseWriter, r *http.Request) {
    validUuid := validateAccessToken(r, w, cfg)
    if validUuid == (uuid.UUID{}) {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    var chirpId uuid.NullUUID
    chirpId.UUID = uuid.MustParse(r.PathValue("chirpID"))
    chirpId.Valid = true
    chirpResult, err := cfg.queries.GetChirpById(r.Context(), chirpId)
    if err != nil {
        http.Error(w, "chirp not found", http.StatusNotFound)
        return
    }

    if chirpResult.UserID != validUuid {
        http.Error(w, "that is not yours", http.StatusForbidden)
        return
    }

    deleteParams := database.DeleteChirpForUserParams {
        ID: chirpId,
        UserID: validUuid,
    }
    cfg.queries.DeleteChirpForUser(r.Context(), deleteParams)
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusNoContent)
}

func (cfg* apiConfig) login(w http.ResponseWriter, r *http.Request)  {
    type body struct {
        Password string `json:"password"`
        Email string `json:"email"`
    }

    type userReturn struct {
        database.User
        Token string `json:"token"`
        RefreshToken string `json:"refresh_token"`
    }

    data, err := io.ReadAll(r.Body)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(data, &rb)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    userRow, err := cfg.queries.GetUser(r.Context(), rb.Email)
    err = auth.CheckPasswordHash(rb.Password, userRow.HashedPassword)
    if err != nil {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    tok, err := auth.MakeJWT(userRow.ID.UUID, cfg.tokenSecret, time.Duration(3600) * time.Second)
    if err != nil {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    // ignoring error, it seems very rare that an error is possible
    refTok, _ := auth.MakeRefreshToken()
    nullTime := sql.NullTime {
            Time: time.Now().Add(60*(24*time.Hour)),
            Valid: true,
    }

    refTokDbParam := database.CreateRefreshTokenParams {
        Token: refTok,
        UserID: userRow.ID.UUID,
        ExpiresAt: nullTime.Time,
    }

    // store the refresh token in the database
    cfg.queries.CreateRefreshToken(r.Context(), refTokDbParam)

    user := userReturn {
        userRow,
        tok,
        refTok,
    }

    d, _ := json.Marshal(user)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg *apiConfig) refreshToken(w http.ResponseWriter, r *http.Request) {
    bt, _ := auth.GetBearerToken(r.Header)

    refreshTokenRow, err := cfg.queries.GetUserFromRefreshToken(r.Context(), bt)
    if err != nil || refreshTokenRow.ExpiresAt.Before(time.Now()) || refreshTokenRow.RevokedAt.Valid == true {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    authToken, err := auth.MakeJWT(refreshTokenRow.UserID, cfg.tokenSecret, time.Duration(3600)*time.Second)
    if err != nil {
        w.Header().Set("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusUnauthorized)
        return
    }
    type out struct {
        Token string `json:"token"`
    }
    outToken := out {
        authToken,
    }

    d, _ := json.Marshal(outToken)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    w.Write(d)
}

func (cfg *apiConfig) chirpyRedPayment(w http.ResponseWriter, r *http.Request) {
    type data struct {
        UserId uuid.UUID `json:"user_id"`
    }

    type body struct {
        Event string `json:"event"`
        Data data `json:"data"`
    }

    bodyData, err := io.ReadAll(r.Body)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    rb := body{}
    err = json.Unmarshal(bodyData, &rb)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusBadRequest)
        w.Write([]byte(`{"error":"something went wrong"}`))
        return
    }

    if rb.Event != "user.upgraded" {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusNoContent)
        return
    }

    param := database.UpdateRedParams {
        ID: uuid.NullUUID{ UUID: rb.Data.UserId, Valid: true, },
        IsChirpyRed: true,
    }

    x, err := cfg.queries.UpdateRed(r.Context(), param)
    if err != nil {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusNotFound)
        return
    }

    fmt.Println(x)

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) revokeRefreshToken(w http.ResponseWriter, r *http.Request) {
    bt, _ := auth.GetBearerToken(r.Header)
    cfg.queries.RevokeRefreshToken(r.Context(), bt)

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusNoContent)
}

func HandleHealthz(writer http.ResponseWriter, request *http.Request)  {
    writer.Header().Set("Content-Type", "text/plain; charset=utf-8") // normal header
    writer.WriteHeader(http.StatusOK)
    writer.Write([]byte("OK"))
}

func scrubMessage(message string) string  {
    tmpWords := strings.Split(message, " ")
    for i, word := range tmpWords {
        tmpWord := strings.ToLower(word)
        switch tmpWord {
        case "kerfuffle", "sharbert", "fornax":
            tmpWords[i] = "****"
        default:
        }
    }
    return strings.Join(tmpWords, " ")
}

func main() {
    godotenv.Load()
    dbURL := os.Getenv("DB_URL")
    platform := os.Getenv("PLATFORM")
    sec := os.Getenv("SECRET")

    db, err := sql.Open("postgres", dbURL)
    if err != nil {
        fmt.Printf("database open failed")
        return
    }
    dbQueries := database.New(db)

    theCounter := apiConfig{}
    theCounter.queries = dbQueries
    theCounter.platform = platform
    theCounter.tokenSecret = sec

    serveMux := http.NewServeMux()
    server := http.Server {
        Handler: serveMux,
        Addr: ":8080",
    }

    serveMux.Handle("/app/", http.StripPrefix("/app",
        theCounter.MiddlewareMetricsInc(http.FileServer(http.Dir(".")))))

    serveMux.HandleFunc("GET /api/healthz", HandleHealthz)
    serveMux.HandleFunc("GET /admin/metrics", theCounter.GetHits)
    serveMux.HandleFunc("POST /api/users", theCounter.createUser)
    serveMux.HandleFunc("POST /admin/reset", theCounter.resetHits)
    serveMux.HandleFunc("POST /api/chirps", theCounter.createChirp)
    serveMux.HandleFunc("GET /api/chirps", theCounter.getChirps)
    serveMux.HandleFunc("GET /api/chirps/{chirpID}", theCounter.getChirpBy)
    serveMux.HandleFunc("POST /api/login", theCounter.login)
    serveMux.HandleFunc("POST /api/refresh", theCounter.refreshToken)
    serveMux.HandleFunc("POST /api/revoke", theCounter.revokeRefreshToken)
    serveMux.HandleFunc("PUT /api/users", theCounter.updateUser)
    serveMux.HandleFunc("DELETE /api/chirps/{chirpID}", theCounter.deleteChirp)
    serveMux.HandleFunc("POST /api/polka/webhooks", theCounter.chirpyRedPayment)
    server.ListenAndServe()
}
