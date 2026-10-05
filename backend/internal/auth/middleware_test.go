package auth

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/google/uuid"
 )

type middlewareStore struct { active bool }
func (s *middlewareStore) FindUserByEmail(context.Context,string)(User,error){ return User{},nil }
func (s *middlewareStore) CreateUser(context.Context,string,string,string)(User,error){ return User{},nil }
func (s *middlewareStore) CreateSession(context.Context,uuid.UUID,string,time.Time)(uuid.UUID,error){ return uuid.New(),nil }
func (s *middlewareStore) IsSessionActive(context.Context,uuid.UUID,time.Time)(bool,error){ return s.active,nil }
func (s *middlewareStore) RevokeSession(context.Context,uuid.UUID,time.Time)error{s.active=false;return nil}

func TestWithAuthAcceptsActiveSession(t *testing.T) {
    now := time.Now().UTC(); uid,sid := uuid.New(),uuid.New()
    manager,err:=NewTokenManager("01234567890123456789012345678901","web-studio-img",time.Hour); if err!=nil{t.Fatal(err)}
    token,_,err:=manager.Issue(uid,sid,now); if err!=nil{t.Fatal(err)}
    store:=&middlewareStore{active:true}
    next:=http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ p,ok:=PrincipalFromContext(r.Context()); if !ok || p.UserID!=uid || p.SessionID!=sid { t.Fatal("principal missing or incorrect") }; w.WriteHeader(http.StatusNoContent) })
    req:=httptest.NewRequest(http.MethodGet,"/api/v1/projects",nil); req.Header.Set("Authorization","Bearer "+token)
    rec:=httptest.NewRecorder(); WithAuth(manager,store,next).ServeHTTP(rec,req)
    if rec.Code!=http.StatusNoContent{t.Fatalf("status=%d",rec.Code)}
}

func TestWithAuthRejectsRevokedSession(t *testing.T) {
    now := time.Now().UTC(); uid,sid := uuid.New(),uuid.New()
    manager,err:=NewTokenManager("01234567890123456789012345678901","web-studio-img",time.Hour); if err!=nil{t.Fatal(err)}
    token,_,err:=manager.Issue(uid,sid,now); if err!=nil{t.Fatal(err)}
    store:=&middlewareStore{active:false}
    req:=httptest.NewRequest(http.MethodGet,"/api/v1/projects",nil); req.Header.Set("Authorization","Bearer "+token)
    rec:=httptest.NewRecorder(); WithAuth(manager,store,http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(http.StatusNoContent)})).ServeHTTP(rec,req)
    if rec.Code!=http.StatusUnauthorized{t.Fatalf("status=%d",rec.Code)}
}