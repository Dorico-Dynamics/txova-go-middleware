package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Dorico-Dynamics/txova-go-core/errors"
)

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return key
}

func generateTestECDSAKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ECDSA key: %v", err)
	}
	return key
}

func createTestToken(t *testing.T, claims *Claims, key any, method jwt.SigningMethod) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	tokenString, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return tokenString
}

func TestNewValidator(t *testing.T) {
	rsaKey := generateTestRSAKey(t)
	ecdsaKey := generateTestECDSAKey(t)

	tests := []struct {
		name    string
		cfg     ValidatorConfig
		wantErr bool
	}{
		{
			name: "RSA public key",
			cfg: ValidatorConfig{
				PublicKey: &rsaKey.PublicKey,
			},
			wantErr: false,
		},
		{
			name: "ECDSA public key",
			cfg: ValidatorConfig{
				PublicKey: &ecdsaKey.PublicKey,
			},
			wantErr: false,
		},
		{
			name: "HMAC secret",
			cfg: ValidatorConfig{
				PublicKey: []byte("secret-key-for-hmac-signing"),
			},
			wantErr: false,
		},
		{
			name: "with issuer and audience",
			cfg: ValidatorConfig{
				PublicKey: &rsaKey.PublicKey,
				Issuer:    "test-issuer",
				Audience:  []string{"test-audience"},
			},
			wantErr: false,
		},
		{
			name:    "nil public key",
			cfg:     ValidatorConfig{},
			wantErr: true,
		},
		{
			name: "unsupported key type",
			cfg: ValidatorConfig{
				PublicKey: "string-is-not-a-valid-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("NewValidator() expected error")
				}
				if !errors.IsValidationError(err) {
					t.Errorf("NewValidator() should return validation error, got %T", err)
				}
				return
			}

			if err != nil {
				t.Errorf("NewValidator() error = %v", err)
				return
			}

			if validator == nil {
				t.Error("NewValidator() returned nil validator")
			}
		})
	}
}

func TestValidator_Validate_RSA(t *testing.T) {
	rsaKey := generateTestRSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	validClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID:   "user-123",
		UserType: "admin",
		Roles:    []string{"admin", "user"},
	}

	isTokenInvalid := func(err error) bool {
		return errors.IsCode(err, errors.CodeTokenInvalid)
	}
	isTokenExpired := func(err error) bool {
		return errors.IsCode(err, errors.CodeTokenExpired)
	}

	tests := []struct {
		name        string
		tokenString string
		wantUserID  string
		wantErr     bool
		checkErr    func(error) bool
	}{
		{
			name:        "valid token",
			tokenString: createTestToken(t, validClaims, rsaKey, jwt.SigningMethodRS256),
			wantUserID:  "user-123",
			wantErr:     false,
		},
		{
			name:        "empty token",
			tokenString: "",
			wantErr:     true,
			checkErr:    isTokenInvalid,
		},
		{
			name:        "malformed token",
			tokenString: "not.a.valid.token",
			wantErr:     true,
			checkErr:    isTokenInvalid,
		},
		{
			name: "expired token",
			tokenString: createTestToken(t, &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				},
			}, rsaKey, jwt.SigningMethodRS256),
			wantErr:  true,
			checkErr: isTokenExpired,
		},
		{
			name: "not valid yet",
			tokenString: createTestToken(t, &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
					NotBefore: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
			}, rsaKey, jwt.SigningMethodRS256),
			wantErr:  true,
			checkErr: isTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := validator.Validate(tt.tokenString)

			if tt.wantErr {
				if err == nil {
					t.Error("Validate() expected error")
					return
				}
				if tt.checkErr != nil && !tt.checkErr(err) {
					t.Errorf("Validate() error type mismatch: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() error = %v", err)
				return
			}

			if claims.UserID != tt.wantUserID {
				t.Errorf("Validate() UserID = %q, want %q", claims.UserID, tt.wantUserID)
			}
		})
	}
}

func TestValidator_Validate_ECDSA(t *testing.T) {
	ecdsaKey := generateTestECDSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &ecdsaKey.PublicKey,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "ecdsa-user",
	}

	tokenString := createTestToken(t, claims, ecdsaKey, jwt.SigningMethodES256)

	result, err := validator.Validate(tokenString)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
		return
	}

	if result.UserID != "ecdsa-user" {
		t.Errorf("Validate() UserID = %q, want %q", result.UserID, "ecdsa-user")
	}
}

func TestValidator_Validate_HMAC(t *testing.T) {
	secret := []byte("test-hmac-secret-key-for-signing")

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: secret,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "hmac-user",
	}

	tokenString := createTestToken(t, claims, secret, jwt.SigningMethodHS256)

	result, err := validator.Validate(tokenString)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
		return
	}

	if result.UserID != "hmac-user" {
		t.Errorf("Validate() UserID = %q, want %q", result.UserID, "hmac-user")
	}
}

func TestValidator_Validate_WrongKey(t *testing.T) {
	rsaKey1 := generateTestRSAKey(t)
	rsaKey2 := generateTestRSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey1.PublicKey,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		UserID: "test-user",
	}

	// Sign with a different key.
	tokenString := createTestToken(t, claims, rsaKey2, jwt.SigningMethodRS256)

	_, err = validator.Validate(tokenString)
	if err == nil {
		t.Error("Validate() expected error for wrong key")
		return
	}

	if !errors.IsCode(err, errors.CodeTokenInvalid) {
		t.Errorf("Validate() expected TokenInvalid error, got %v", err)
	}
}

func TestValidator_Validate_WithIssuer(t *testing.T) {
	rsaKey := generateTestRSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
		Issuer:    "expected-issuer",
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	tests := []struct {
		name    string
		issuer  string
		wantErr bool
	}{
		{
			name:    "correct issuer",
			issuer:  "expected-issuer",
			wantErr: false,
		},
		{
			name:    "wrong issuer",
			issuer:  "wrong-issuer",
			wantErr: true,
		},
		{
			name:    "empty issuer",
			issuer:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Issuer:    tt.issuer,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
				UserID: "test-user",
			}

			tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

			_, err := validator.Validate(tokenString)

			if tt.wantErr && err == nil {
				t.Error("Validate() expected error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidator_Validate_WithAudience(t *testing.T) {
	rsaKey := generateTestRSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
		Audience:  []string{"expected-audience"},
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	tests := []struct {
		name     string
		audience []string
		wantErr  bool
	}{
		{
			name:     "correct audience",
			audience: []string{"expected-audience"},
			wantErr:  false,
		},
		{
			name:     "multiple audiences including correct",
			audience: []string{"other-audience", "expected-audience"},
			wantErr:  false,
		},
		{
			name:     "wrong audience",
			audience: []string{"wrong-audience"},
			wantErr:  true,
		},
		{
			name:     "empty audience",
			audience: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Audience:  tt.audience,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
				UserID: "test-user",
			}

			tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

			_, err := validator.Validate(tokenString)

			if tt.wantErr && err == nil {
				t.Error("Validate() expected error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidator_Validate_WithMultipleConfiguredAudiences(t *testing.T) {
	rsaKey := generateTestRSAKey(t)

	// Configure validator with multiple acceptable audiences (any-of matching).
	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
		Audience:  []string{"api", "web", "mobile"},
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	tests := []struct {
		name          string
		tokenAudience []string
		wantErr       bool
	}{
		{
			name:          "token has first configured audience",
			tokenAudience: []string{"api"},
			wantErr:       false,
		},
		{
			name:          "token has second configured audience",
			tokenAudience: []string{"web"},
			wantErr:       false,
		},
		{
			name:          "token has third configured audience",
			tokenAudience: []string{"mobile"},
			wantErr:       false,
		},
		{
			name:          "token has multiple audiences including one configured",
			tokenAudience: []string{"other", "mobile", "extra"},
			wantErr:       false,
		},
		{
			name:          "token has all configured audiences",
			tokenAudience: []string{"api", "web", "mobile"},
			wantErr:       false,
		},
		{
			name:          "token has none of the configured audiences",
			tokenAudience: []string{"other", "unknown"},
			wantErr:       true,
		},
		{
			name:          "token has empty audience",
			tokenAudience: nil,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Audience:  tt.tokenAudience,
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
				UserID: "test-user",
			}

			tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

			_, err := validator.Validate(tokenString)

			if tt.wantErr && err == nil {
				t.Error("Validate() expected error for invalid audience")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestValidator_Validate_AllSigningMethods(t *testing.T) {
	rsaKey := generateTestRSAKey(t)
	ecdsaKey := generateTestECDSAKey(t)
	hmacSecret := []byte("test-hmac-secret-key-for-signing")

	tests := []struct {
		name   string
		key    any
		pubKey any
		method jwt.SigningMethod
	}{
		{"RS256", rsaKey, &rsaKey.PublicKey, jwt.SigningMethodRS256},
		{"RS384", rsaKey, &rsaKey.PublicKey, jwt.SigningMethodRS384},
		{"RS512", rsaKey, &rsaKey.PublicKey, jwt.SigningMethodRS512},
		{"ES256", ecdsaKey, &ecdsaKey.PublicKey, jwt.SigningMethodES256},
		{"HS256", hmacSecret, hmacSecret, jwt.SigningMethodHS256},
		{"HS384", hmacSecret, hmacSecret, jwt.SigningMethodHS384},
		{"HS512", hmacSecret, hmacSecret, jwt.SigningMethodHS512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator, err := NewValidator(ValidatorConfig{
				PublicKey: tt.pubKey,
			})
			if err != nil {
				t.Fatalf("NewValidator() error = %v", err)
			}

			claims := &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
					IssuedAt:  jwt.NewNumericDate(time.Now()),
				},
				UserID: "test-user-" + tt.name,
			}

			tokenString := createTestToken(t, claims, tt.key, tt.method)

			result, err := validator.Validate(tokenString)
			if err != nil {
				t.Errorf("Validate() error = %v", err)
				return
			}

			if result.UserID != claims.UserID {
				t.Errorf("Validate() UserID = %q, want %q", result.UserID, claims.UserID)
			}
		})
	}
}

func TestValidator_Validate_ClaimsPopulated(t *testing.T) {
	rsaKey := generateTestRSAKey(t)

	validator, err := NewValidator(ValidatorConfig{
		PublicKey: &rsaKey.PublicKey,
	})
	if err != nil {
		t.Fatalf("NewValidator() error = %v", err)
	}

	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "subject-123",
			Issuer:    "test-issuer",
			Audience:  []string{"aud1", "aud2"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        "jwt-id-123",
		},
		UserID:      "user-456",
		UserType:    "driver",
		Roles:       []string{"driver", "verified"},
		Permissions: []string{"trips:read", "trips:write"},
	}

	tokenString := createTestToken(t, claims, rsaKey, jwt.SigningMethodRS256)

	result, err := validator.Validate(tokenString)
	if err != nil {
		t.Errorf("Validate() error = %v", err)
		return
	}

	if result.Subject != claims.Subject {
		t.Errorf("Subject = %q, want %q", result.Subject, claims.Subject)
	}
	if result.Issuer != claims.Issuer {
		t.Errorf("Issuer = %q, want %q", result.Issuer, claims.Issuer)
	}
	if result.UserID != claims.UserID {
		t.Errorf("UserID = %q, want %q", result.UserID, claims.UserID)
	}
	if result.UserType != claims.UserType {
		t.Errorf("UserType = %q, want %q", result.UserType, claims.UserType)
	}
	if len(result.Roles) != len(claims.Roles) {
		t.Errorf("Roles length = %d, want %d", len(result.Roles), len(claims.Roles))
	}
	if len(result.Permissions) != len(claims.Permissions) {
		t.Errorf("Permissions length = %d, want %d", len(result.Permissions), len(claims.Permissions))
	}
}
