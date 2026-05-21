package speck

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingToken   = errors.New("missing authentication token")
	ErrMissingScope   = errors.New("missing required scope")
	ErrForbiddenScope = errors.New("forbidden scope present")
)

// TokenPayload is parsed JWT payload data exposed to route handlers.
type TokenPayload struct {
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	NotBefore time.Time
	Claims    map[string]any
}

func (payload *TokenPayload) HasScope(scope string) bool {
	if payload == nil {
		return false
	}

	for _, existing := range payload.Scopes {
		if existing == scope {
			return true
		}
	}
	return false
}

func (payload *TokenPayload) EnsureScope(scope string) error {
	if payload == nil {
		return ErrMissingToken
	}
	if !payload.HasScope(scope) {
		return fmt.Errorf("%w: %s", ErrMissingScope, scope)
	}
	return nil
}

func (payload *TokenPayload) ForbidScope(scope string) error {
	if payload == nil {
		return nil
	}
	if payload.HasScope(scope) {
		return fmt.Errorf("%w: %s", ErrForbiddenScope, scope)
	}
	return nil
}

func (payload *TokenPayload) Claim(name string) (any, bool) {
	if payload == nil || payload.Claims == nil {
		return nil, false
	}
	value, ok := payload.Claims[name]
	return value, ok
}

type jwtAuthenticator struct {
	signingKey    []byte
	allowUnsigned bool
	now           func() time.Time
}

func newJWTAuthenticator(signingKey []byte) *jwtAuthenticator {
	return &jwtAuthenticator{
		signingKey: append([]byte(nil), signingKey...),
		now:        time.Now,
	}
}

func (authenticator *jwtAuthenticator) AllowUnsigned() *jwtAuthenticator {
	authenticator.allowUnsigned = true
	return authenticator
}

func (authenticator *jwtAuthenticator) parse(token string) (*TokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	signedContent := parts[0] + "." + parts[1]

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("invalid jwt header encoding")
	}
	var header struct {
		Algorithm string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, errors.New("invalid jwt header")
	}
	if err := authenticator.validateSignature(header.Algorithm, signedContent, parts[2]); err != nil {
		return nil, err
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid jwt payload encoding")
	}

	payload, err := parseTokenPayload(payloadBytes)
	if err != nil {
		return nil, err
	}
	if err := authenticator.validateTimes(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (authenticator *jwtAuthenticator) validateSignature(algorithm string, signedContent string, signature string) error {
	switch algorithm {
	case "HS256":
		expectedSignature := signHS256([]byte(signedContent), authenticator.signingKey)
		actualSignature, err := base64.RawURLEncoding.DecodeString(signature)
		if err != nil {
			return errors.New("invalid jwt signature encoding")
		}
		if !hmac.Equal(actualSignature, expectedSignature) {
			return errors.New("invalid jwt signature")
		}
		return nil
	case "none":
		if !authenticator.allowUnsigned {
			return errors.New("unsigned jwt tokens are not allowed")
		}
		if signature != "" {
			return errors.New("unsigned jwt token must not include a signature")
		}
		return nil
	default:
		return errors.New("unsupported jwt algorithm")
	}
}

func signHS256(data []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func parseTokenPayload(payloadBytes []byte) (*TokenPayload, error) {
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, errors.New("invalid jwt payload")
	}

	payload := &TokenPayload{Claims: claims}
	if subject, ok := claims["sub"].(string); ok {
		payload.Subject = subject
	}
	payload.Scopes = scopesFromClaims(claims)
	payload.ExpiresAt = timeFromClaim(claims["exp"])
	payload.IssuedAt = timeFromClaim(claims["iat"])
	payload.NotBefore = timeFromClaim(claims["nbf"])
	return payload, nil
}

func scopesFromClaims(claims map[string]any) []string {
	seen := map[string]bool{}
	var scopes []string
	addScope := func(scope string) {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			return
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}

	if scopeString, ok := claims["scope"].(string); ok {
		for _, scope := range strings.Fields(scopeString) {
			addScope(scope)
		}
	}
	if scopeValues, ok := claims["scopes"].([]any); ok {
		for _, scopeValue := range scopeValues {
			if scope, ok := scopeValue.(string); ok {
				addScope(scope)
			}
		}
	}
	return scopes
}

func timeFromClaim(value any) time.Time {
	number, ok := value.(float64)
	if !ok || number == 0 {
		return time.Time{}
	}
	return time.Unix(int64(number), 0)
}

func (authenticator *jwtAuthenticator) validateTimes(payload *TokenPayload) error {
	now := authenticator.now()
	if !payload.ExpiresAt.IsZero() && !now.Before(payload.ExpiresAt) {
		return errors.New("jwt token has expired")
	}
	if !payload.NotBefore.IsZero() && now.Before(payload.NotBefore) {
		return errors.New("jwt token is not valid yet")
	}
	return nil
}

func encodeJWTPart(value any) string {
	bytes, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func unsignedJWTForTest(claims map[string]any) string {
	return encodeJWTPart(map[string]any{"alg": "none", "typ": "JWT"}) + "." + encodeJWTPart(claims) + "."
}

// SignJWT signs claims with HS256.
func SignJWT(claims map[string]any, signingKey []byte) string {
	header := encodeJWTPart(map[string]any{"alg": "HS256", "typ": "JWT"})
	payload := encodeJWTPart(claims)
	signedContent := header + "." + payload
	signature := base64.RawURLEncoding.EncodeToString(signHS256([]byte(signedContent), signingKey))
	return signedContent + "." + signature
}
