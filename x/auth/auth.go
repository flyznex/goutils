package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/flyznex/gois"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
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
	AuthManager struct {
		Validator *gois.JWTValidator
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

// NewAuthManager create new AuthManager instance
func NewAuthManager(cfg ConfigAuth) *AuthManager {
	m := jose.RS256
	if cfg.MethodSignature != "" {
		m = jose.SignatureAlgorithm(cfg.MethodSignature)
	}
	authClient := gois.NewJWKClient(gois.JWKClientOptions{URI: cfg.IdentityServerURI}, nil)
	configuration := gois.NewConfiguration(authClient, cfg.Audiences, cfg.Issuer, m)
	validator := gois.NewValidator(configuration, nil)
	return &AuthManager{
		Validator: validator,
	}
}

func (am *AuthManager) Authenticate() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		hfn := func(w http.ResponseWriter, r *http.Request) {
			raw := ""
			if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[0:7], "BEARER ") {
				raw = h[7:]
			}
			if raw == "" {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			ctx := context.WithValue(r.Context(), JWTToken, raw)
			token, err := am.Validator.ValidateRequest(r)
			if err != nil {
				http.Error(w, http.StatusText(401), 401)
				return
			}
			ctx = context.WithValue(ctx, TokenKey, token)
			ctx = buildContextWithValue(ctx, am, token)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(hfn)
	}
}
func buildContextWithValue(ctx context.Context, am *AuthManager, token *jwt.JSONWebToken) context.Context {
	claims := map[string]interface{}{}
	err := am.Validator.Claims(token, &claims)
	if err != nil {
		return ctx
	}
	ctx = context.WithValue(ctx, IdentityKey, claims)
	roles := getRoleFromClaims(claims)
	ctx = context.WithValue(ctx, RolesKey, roles)
	userID := getUserIDFromClaims(claims)
	ctx = context.WithValue(ctx, UserIDKey, userID)
	return ctx
}

// Context keys
var (
	TokenKey    = &contextKey{"Token"}
	JWTToken    = &contextKey{"JWTToken"}
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
			userID := getUserIDFromClaims(claims)
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
func getUserIDFromClaims(claims map[string]interface{}) string {
	sub, ok := claims["sub"]
	if !ok {
		return ""
	}
	return sub.(string)
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
