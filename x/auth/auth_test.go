package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	auth0 "github.com/auth0-community/go-auth0"
	"github.com/go-chi/chi"
	jose "gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"
)

var (
	emptyAudience = []string{}
	emptyIssuer   = ""

	defaultAudience = []string{"audience"}
	defaultIssuer   = "issuer"

	// The default generated token by Chrome jwt extension
	defaultSecret = []byte("secret")
	//defaultSecretProvider = auth0.NewKeyProvider(defaultSecret)

	defaultSecretRS256 = genRSASSAJWK(jose.RS256, "")
	//defaultSecretProviderRS256 = auth0.NewKeyProvider(defaultSecretRS256.Public())

	defaultSecretES384 = genECDSAJWK(jose.ES384, "")
	//defaultSecretProviderES384 = auth0.NewKeyProvider(defaultSecretES384.Public())
	Timeout       = time.Second * 10
	defaultMethod = jose.RS256
)

func genRSASSAJWK(sigAlg jose.SignatureAlgorithm, kid string) jose.JSONWebKey {
	var bits int
	if sigAlg == jose.RS256 {
		bits = 2048
	}
	if sigAlg == jose.RS512 {
		bits = 4096
	}

	key, _ := rsa.GenerateKey(rand.Reader, bits)

	jsonWebKey := jose.JSONWebKey{
		Key:       key,
		KeyID:     kid,
		Use:       "sig",
		Algorithm: string(sigAlg),
	}

	return jsonWebKey
}

func genECDSAJWK(sigAlg jose.SignatureAlgorithm, kid string) jose.JSONWebKey {
	var c elliptic.Curve
	if sigAlg == jose.ES256 {
		c = elliptic.P256()
	}
	if sigAlg == jose.ES384 {
		c = elliptic.P384()
	}

	key, _ := ecdsa.GenerateKey(c, rand.Reader)

	jsonWebKey := jose.JSONWebKey{
		Key:       key,
		KeyID:     kid,
		Algorithm: string(sigAlg),
	}

	return jsonWebKey
}

func getTestToken(audience []string, issuer string, expTime time.Time, alg jose.SignatureAlgorithm, key interface{}) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		panic(err)
	}

	cl := jwt.Claims{
		Issuer:   issuer,
		Audience: audience,
		IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		Expiry:   jwt.NewNumericDate(expTime),
	}

	raw, err := jwt.Signed(signer).Claims(cl).CompactSerialize()
	if err != nil {
		panic(err)
	}
	return raw
}

func getTestTokenWithKid(audience []string, issuer string, expTime time.Time, alg jose.SignatureAlgorithm, key interface{}, kid string) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, (&jose.SignerOptions{ExtraHeaders: map[jose.HeaderKey]interface{}{"kid": kid}}).WithType("JWT"))
	if err != nil {
		panic(err)
	}

	cl := jwt.Claims{
		Issuer:   issuer,
		Audience: audience,
		IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		Expiry:   jwt.NewNumericDate(expTime),
		Subject:  "user-id",
	}
	rl := make(map[string]interface{})
	rl["role"] = []string{"admin", "xadmin", "user"}
	raw, err := jwt.Signed(signer).Claims(cl).Claims(rl).CompactSerialize()
	if err != nil {
		panic(err)
	}
	return raw
}

func genNewTestServer(genJWKS bool) (auth0.JWKClientOptions, string, string, error) {
	// Generate JWKs
	jsonWebKeyRS256 := genRSASSAJWK(jose.RS256, "keyRS256")
	jsonWebKeyES384 := genECDSAJWK(jose.ES384, "keyES384")

	// Generate JWKS
	jwks := auth0.JWKS{
		Keys: []jose.JSONWebKey{},
	}
	if genJWKS {
		jwks = auth0.JWKS{
			Keys: []jose.JSONWebKey{jsonWebKeyRS256.Public(), jsonWebKeyES384.Public()},
		}
	}
	value, err := json.Marshal(&jwks)

	// Generate Tokens
	tokenRS256 := getTestTokenWithKid(defaultAudience, defaultIssuer, time.Now().Add(24*time.Hour), jose.RS256, jsonWebKeyRS256, "keyRS256")
	tokenES384 := getTestTokenWithKid(defaultAudience, defaultIssuer, time.Now().Add(24*time.Hour), jose.ES384, jsonWebKeyES384, "keyES384")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/jwk-set+json")
		fmt.Fprintln(w, string(value))
	}))
	return auth0.JWKClientOptions{URI: ts.URL}, tokenRS256, tokenES384, err
}

func TestMiddlewareHandleWithToken(t *testing.T) {
	opts, token, _, err := genNewTestServer(true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	r := chi.NewRouter()
	cfg := ConfigAuth{
		Issuer:            defaultIssuer,
		Audiences:         defaultAudience,
		IdentityServerURI: opts.URI,
	}
	auth := New(cfg)
	r.Use(Authenticator(auth), RequireRoles("user"))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	gServer := httptest.NewServer(r)
	url := fmt.Sprintf("%s/", gServer.URL)
	req, _ := http.NewRequest("GET", url, nil)
	headerValue := fmt.Sprintf("Bearer %s", token)
	req.Header.Add("Authorization", headerValue)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   Timeout * 300,
		Transport: tr,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if resp.StatusCode != 200 {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}
}

func TestMiddlewareHandleWithoutToken(t *testing.T) {
	opts, _, _, err := genNewTestServer(true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	r := chi.NewRouter()
	cfg := ConfigAuth{
		Issuer:            defaultIssuer,
		Audiences:         defaultAudience,
		IdentityServerURI: opts.URI,
	}
	auth := New(cfg)
	r.Use(Authenticator(auth))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	gServer := httptest.NewServer(r)
	url := fmt.Sprintf("%s/", gServer.URL)
	req, _ := http.NewRequest("GET", url, nil)
	// headerValue := fmt.Sprintf("Bearer %s", token)
	// req.Header.Add("Authorization", headerValue)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   Timeout,
		Transport: tr,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if resp.StatusCode != 401 {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}
}

func TestMiddlewareHandleWithTokenNoValidRole(t *testing.T) {
	opts, token, _, err := genNewTestServer(true)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	r := chi.NewRouter()
	cfg := ConfigAuth{
		Issuer:            defaultIssuer,
		Audiences:         defaultAudience,
		IdentityServerURI: opts.URI,
	}
	auth := New(cfg)
	r.Use(Authenticator(auth), RequireRoles("manager"))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("welcome"))
	})
	gServer := httptest.NewServer(r)
	url := fmt.Sprintf("%s/", gServer.URL)
	req, _ := http.NewRequest("GET", url, nil)
	headerValue := fmt.Sprintf("Bearer %s", token)
	req.Header.Add("Authorization", headerValue)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{
		Timeout:   Timeout,
		Transport: tr,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if resp.StatusCode != 401 {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}
}
