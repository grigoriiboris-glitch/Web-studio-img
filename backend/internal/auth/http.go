package auth

import (
    "encoding/json"
    "errors"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/google/uuid"
 )

type HTTPHandler struct { service *Service; tokens *TokenManager; store Store }

func NewHTTPHandler(service *Service, tokens *TokenManager, store Store) (*HTTPHandler, error) {
    if service == nil || tokens == nil || store == nil { return nil, errors.New("auth http handler requires service, token manager and store") }
    return &HTTPHandler{service: service, tokens: tokens, store: store}, nil
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
    mux.HandleFunc("POST /api/v1/auth/register", h.register)
    mux.HandleFunc("POST /api/v1/auth/login", h.login)
    mux.HandleFunc("GET /api/v1/auth/me", WithAuth(h.tokens, h.store, http.HandlerFunc(h.me)).ServeHTTP)
    mux.HandleFunc("POST /api/v1/auth/logout", WithAuth(h.tokens, h.store, http.HandlerFunc(h.logout)).ServeHTTP)
}

type credentialsInput struct { Email string; Password string; Name string }

func (h *HTTPHandler) register(w http.ResponseWriter, r *http.Request) {
    var input credentialsInput
    if err := decodeJSON(r, &input); err != nil || strings.TrimSpace(input.Name) == "" { writeAuthError(w,400,"invalid_request","email, name and password are required"); return }
    user, token, expires, err := h.service.Register(r.Context(), input.Email, input.Name, input.Password)
    if err != nil {
        if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") { writeAuthError(w,409,"email_already_registered","email is already registered"); return }
        if strings.Contains(err.Error(), "password must") || strings.Contains(err.Error(), "required") { writeAuthError(w,400,"invalid_request",err.Error()); return }
        writeAuthError(w,500,"registration_failed","could not register user"); return
    }
    writeAuthJSON(w,201,map[string]any{"user":publicUser(user),"token":token,"expires_at":expires})
}

func (h *HTTPHandler) login(w http.ResponseWriter, r *http.Request) {
    var input credentialsInput
    if err := decodeJSON(r, &input); err != nil { writeAuthError(w,400,"invalid_request","email and password are required"); return }
    user, token, expires, err := h.service.Login(r.Context(), input.Email, input.Password)
    if errors.Is(err, ErrInvalidCredentials) { writeAuthError(w,401,"invalid_credentials","invalid email or password"); return }
    if err != nil { writeAuthError(w,500,"login_failed","could not log in"); return }
    writeAuthJSON(w,200,map[string]any{"user":publicUser(user),"token":token,"expires_at":expires})
}

func (h *HTTPHandler) me(w http.ResponseWriter, r *http.Request) {
    principal, ok := PrincipalFromContext(r.Context()); if !ok { writeAuthError(w,401,"unauthorized","authentication required"); return }
    user, err := h.userByID(r, principal.UserID); if err != nil { writeAuthError(w,401,"unauthorized","user session is no longer valid"); return }
    writeAuthJSON(w,200,publicUser(user))
}

func (h *HTTPHandler) logout(w http.ResponseWriter, r *http.Request) {
    principal, ok := PrincipalFromContext(r.Context()); if !ok { writeAuthError(w,401,"unauthorized","authentication required"); return }
    if err := h.service.Logout(r.Context(), principal.SessionID); err != nil { writeAuthError(w,500,"logout_failed","could not log out"); return }
    w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) userByID(r *http.Request, id uuid.UUID) (User,error) {
    sqlStore, ok := h.store.(*SQLStore); if !ok { return User{}, errors.New("auth store does not support user lookup by id") }
    var user User; var password sql.NullString
    err := sqlStore.db.QueryRowContext(r.Context(), "SELECT id,email,name,password_hash FROM users WHERE id=$1 LIMIT 1", id).Scan(&user.ID,&user.Email,&user.Name,&password)
    user.PasswordHash = password.String
    return user, err
}

func publicUser(user User) map[string]any { return map[string]any{"id":user.ID,"email":user.Email,"name":user.Name} }

func decodeJSON(r *http.Request, value any) error {
    decoder := json.NewDecoder(io.LimitReader(r.Body,1<<20)); decoder.DisallowUnknownFields()
    if err := decoder.Decode(value); err != nil { return err }; var extra any; if err := decoder.Decode(&extra); err != io.EOF { return errors.New("request body must contain one JSON object") }; return nil
}

func writeAuthJSON(w http.ResponseWriter,status int,value any) { w.Header().Set("Content-Type","application/json"); w.WriteHeader(status); _=json.NewEncoder(w).Encode(value) }
func writeAuthError(w http.ResponseWriter,status int,code,message string) { writeAuthJSON(w,status,map[string]any{"error":map[string]string{"code":code,"message":message,"request_id":uuid.NewString()}}) }