package auth

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	CookieName = "mountest_admin"

	RoleSuperadmin = "superadmin"
	RoleEditor     = "editor"
)

type ctxKey string

const (
	adminIDKey   ctxKey = "adminID"
	adminRoleKey ctxKey = "adminRole"
)

type Claims struct {
	AdminID string `json:"sub"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

type Service struct {
	secret []byte
	secure bool
}

func NewService(secret string, secure bool) *Service {
	return &Service{secret: []byte(secret), secure: secure}
}

func (s *Service) Issue(adminID uuid.UUID, role string) (string, time.Time, error) {
	exp := time.Now().Add(7 * 24 * time.Hour)
	claims := Claims{
		AdminID: adminID.String(),
		Role:    role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(s.secret)
	return signed, exp, err
}

func (s *Service) Parse(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func (s *Service) SetCookie(w http.ResponseWriter, value string, exp time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieName)
		if err != nil || c.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := s.Parse(c.Value)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), adminIDKey, claims.AdminID)
		ctx = context.WithValue(ctx, adminRoleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireSuperadmin отбивает запрос 403, если у пользователя роль не superadmin.
// Должен использоваться ПОСЛЕ Middleware.
func RequireSuperadmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role, _ := AdminRoleFrom(r.Context())
		if role != RoleSuperadmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AdminIDFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(adminIDKey).(string)
	return v, ok
}

func AdminRoleFrom(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(adminRoleKey).(string)
	return v, ok
}

// IsSuperadmin — удобный шорткат для бизнес-логики хендлеров.
func IsSuperadmin(ctx context.Context) bool {
	role, _ := AdminRoleFrom(ctx)
	return role == RoleSuperadmin
}
