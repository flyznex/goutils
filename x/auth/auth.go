package auth

import (
	"context"
	"net/http"

	"github.com/flyznex/gois"
	jose "gopkg.in/square/go-jose.v2"
)

//AuthModel auth model context
type (
	AuthModel struct {
		Options         gois.JWKClientOptions
		Audience        []string
		Issuer          string
		MethodSignature jose.SignatureAlgorithm
	}
	ConfigAuth struct {
		Issuer            string
		Audiences         []string
		IdentityServerURI string
		MethodSignature   string
	}
)
type contextKey struct {
	name string
}

//New new AuthModel with default method RS256
func New(cfg ConfigAuth) *AuthModel {
	m := jose.RS256
	if cfg.MethodSignature != "" {
		m = jose.SignatureAlgorithm(cfg.MethodSignature)
	}
	return &AuthModel{
		Audience:        cfg.Audiences,
		Issuer:          cfg.Issuer,
		Options:         gois.JWKClientOptions{URI: cfg.IdentityServerURI},
		MethodSignature: m,
	}
}

// Context keys
var (
	TokenKey    = &contextKey{"Token"}
	IdentityKey = &contextKey{"Identity"}
	UserIDKey   = &contextKey{"UserID"}
	RolesKey    = &contextKey{"Roles"}
)

//Authenticator middleware
func Authenticator(auth *AuthModel) func(http.Handler) http.Handler {
	authClient := gois.NewJWKClient(auth.Options, nil)
	configuration := gois.NewConfiguration(authClient, auth.Audience, auth.Issuer, auth.MethodSignature)
	validator := gois.NewValidator(configuration, nil)
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			token, err := validator.ValidateRequest(r)
			if err != nil {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			ctx := r.Context()
			//ctx = NewContext(ctx, token, err)
			ctx = context.WithValue(ctx, TokenKey, token)
			claims := map[string]interface{}{}
			err = validator.Claims(token, &claims)
			if err != nil {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			ctx = context.WithValue(ctx, IdentityKey, claims)
			roles := getRoleFromClaims(claims)
			ctx = context.WithValue(ctx, RolesKey, roles)
			userID, ok := getUserIDFromClaims(claims)
			if !ok {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			ctx = context.WithValue(ctx, UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(hfn)
	}
}



func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			userRoles := ctx.Value(RolesKey).(map[string]string)
			access := false
			for _, rr := range roles {
				_, ok := userRoles[rr]
				if ok {
					access = ok
					break
				}
			}
			if access {
				next.ServeHTTP(w, r.WithContext(ctx))
			} else {
				http.Error(w, http.StatusText(401), 401)
				return
			}

		})
	}
}


func GetUserNameFromContext(ctx context.Context) string {
	claimsCtx := ctx.Value(IdentityKey)
	if claimsCtx == nil {
		return ""
	}
	claims := claimsCtx.(map[string]interface{})
	name, ok := claims["name"]
	if !ok {
		return ""
	}
	return name.(string)
}


func GetUserIDFromContext(ctx context.Context) string {
	userId := ctx.Value(UserIDKey)
	if userId == nil {
		return ""
	}
	return userId.(string)
}
// internal functions
func getUserIDFromClaims(claims map[string]interface{}) (string, bool) {
	sub, ok := claims["sub"]
	if !ok {
		return "", false
	}
	return sub.(string), ok
}

func getRoleFromClaims(claims map[string]interface{}) map[string]string {
	roles := map[string]string{}
	if rc, ok := claims["role"]; ok {
		switch v := rc.(type) {
		case string:
			roles[v] = v
		case []interface{}:
			for _, r := range v {
				rs := r.(string)
				roles[rs] = rs
			}
		}

	}
	return roles
}
