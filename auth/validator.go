package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	stderrors "errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
)

// TokenValidator is an interface for token validation.
// Implementations can add additional checks beyond JWT signature/expiry
// (e.g., token blacklist, session validity).
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string) (*Claims, error)
}

// Validator validates JWT tokens.
type Validator struct {
	publicKey any
	issuer    string
	audience  []string
}

// ValidatorConfig holds configuration for the JWT validator.
type ValidatorConfig struct {
	// PublicKey is the public key for verifying token signatures.
	// Can be *rsa.PublicKey, *ecdsa.PublicKey, or []byte for HMAC.
	PublicKey any

	// Issuer is the expected issuer claim (optional).
	Issuer string

	// Audience is the expected audience claim(s) (optional).
	Audience []string
}

// NewValidator creates a new JWT validator with the given configuration.
func NewValidator(cfg ValidatorConfig) (*Validator, error) {
	if cfg.PublicKey == nil {
		return nil, errors.ValidationError("public key is required")
	}

	// Validate the key type.
	switch cfg.PublicKey.(type) {
	case *rsa.PublicKey, *ecdsa.PublicKey, []byte:
		// Valid key types.
	default:
		return nil, errors.ValidationErrorf("unsupported key type: %T", cfg.PublicKey)
	}

	return &Validator{
		publicKey: cfg.PublicKey,
		issuer:    cfg.Issuer,
		audience:  cfg.Audience,
	}, nil
}

// Validate parses and validates a JWT token string.
// Returns the claims if valid, or an error if invalid.
func (v *Validator) Validate(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.TokenInvalid("token is empty")
	}

	// Parse options.
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{
			jwt.SigningMethodRS256.Alg(),
			jwt.SigningMethodRS384.Alg(),
			jwt.SigningMethodRS512.Alg(),
			jwt.SigningMethodES256.Alg(),
			jwt.SigningMethodES384.Alg(),
			jwt.SigningMethodES512.Alg(),
			jwt.SigningMethodHS256.Alg(),
			jwt.SigningMethodHS384.Alg(),
			jwt.SigningMethodHS512.Alg(),
		}),
	}

	if v.issuer != "" {
		parserOpts = append(parserOpts, jwt.WithIssuer(v.issuer))
	}

	// Note: We handle audience validation manually after parsing to support
	// any-of matching (token must contain at least one of the configured audiences).

	// Parse the token.
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return v.publicKey, nil
	}, parserOpts...)

	if err != nil {
		return nil, v.mapError(err)
	}

	if !token.Valid {
		return nil, errors.TokenInvalid("token is invalid")
	}

	// Validate audience (any-of matching): token must contain at least one configured audience.
	if len(v.audience) > 0 {
		if !v.validateAudience(claims) {
			return nil, errors.TokenInvalid("token audience does not match any configured audience")
		}
	}

	return claims, nil
}

// ValidateToken implements TokenValidator using stateless JWT validation.
func (v *Validator) ValidateToken(_ context.Context, tokenString string) (*Claims, error) {
	return v.Validate(tokenString)
}

// validateAudience checks if the token's audience contains at least one of the configured audiences.
func (v *Validator) validateAudience(claims *Claims) bool {
	tokenAudiences, err := claims.GetAudience()
	if err != nil || len(tokenAudiences) == 0 {
		return false
	}

	// Build a set of token audiences for O(1) lookup.
	tokenAudSet := make(map[string]bool, len(tokenAudiences))
	for _, aud := range tokenAudiences {
		tokenAudSet[aud] = true
	}

	// Check if any configured audience is present in the token.
	for _, configuredAud := range v.audience {
		if tokenAudSet[configuredAud] {
			return true
		}
	}

	return false
}

// mapError maps JWT library errors to Txova errors.
func (v *Validator) mapError(err error) *errors.AppError {
	switch {
	case stderrors.Is(err, jwt.ErrTokenExpired):
		return errors.TokenExpired("token has expired")
	case stderrors.Is(err, jwt.ErrTokenNotValidYet):
		return errors.TokenInvalid("token is not valid yet")
	case stderrors.Is(err, jwt.ErrTokenMalformed):
		return errors.TokenInvalid("token is malformed")
	case stderrors.Is(err, jwt.ErrSignatureInvalid):
		return errors.TokenInvalid("invalid token signature")
	case stderrors.Is(err, jwt.ErrTokenSignatureInvalid):
		return errors.TokenInvalid("invalid token signature")
	default:
		return errors.TokenInvalid(fmt.Sprintf("token validation failed: %v", err))
	}
}
