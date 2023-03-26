package utils

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

type User struct {
	Id          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name" db:"display_name"`
	Avatar      string `json:"avatar"`
}

type UserWithClaims struct {
	User User `json:"user"`
	jwt.RegisteredClaims
}

func GetExistingUser(db *sqlx.DB, email string) (*User, error) {
	// Find if user already exists in DB
	user := &User{}
	err := db.Get(user, "SELECT id, email, username, display_name, COALESCE(avatar, '') avatar FROM User WHERE email = ?", email)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if user.Id == "" {
		return nil, nil
	}

	return user, nil
}

type GithubUserData struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarUrl string `json:"avatar_url"`
}

func GetGithubUserInfo(code string, config *oauth2.Config) (*GithubUserData, error) {
	// Exchange callback token for bearer token
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return nil, err
	}

	// Get user data
	client := config.Client(context.Background(), token)
	response, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	userData := &GithubUserData{}
	json.NewDecoder(response.Body).Decode(userData)

	return userData, nil
}

type RegisteringJWTClaims struct {
	UserData GithubUserData `json:"user_data"`
	jwt.RegisteredClaims
}

// Useful for coookies
const HOUR_IN_SECONDS = int(time.Hour / time.Second)
const REGISTERING_JWT_COOKIE_NAME = "registering-data"
const AUTH_TOKEN_JWT_COOKIE_NAME = "auth-token"

func CreateRegisteringJWT(githubUserData *GithubUserData) (*fiber.Cookie, error) {
	claims := RegisteringJWTClaims{
		*githubUserData,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(20 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "pigeon-api",
		},
	}

	signingKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(os.Getenv("JWT_SECRET_KEY")))

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(signingKey)

	if err != nil {
		return nil, err
	}

	return &fiber.Cookie{
		Name:     REGISTERING_JWT_COOKIE_NAME,
		Value:    tokenString,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		MaxAge:   HOUR_IN_SECONDS * (1 / 3),
	}, nil
}

func DecodeRegisteringJWT(cookie string) (*GithubUserData, error) {
	token, err := jwt.ParseWithClaims(cookie, &RegisteringJWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseRSAPublicKeyFromPEM([]byte(os.Getenv("JWT_PUBLIC_KEY")))
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*RegisteringJWTClaims)
	if !ok {
		return nil, err
	}

	return &claims.UserData, nil
}

func CreateJWTCookieForUser(user *User) (*fiber.Cookie, error) {
	claims := UserWithClaims{
		*user,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(4 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "pigeon-api",
		},
	}
	signingKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(os.Getenv("JWT_SECRET_KEY")))

	if err != nil {
		return nil, err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(signingKey)

	if err != nil {
		return nil, err
	}

	return &fiber.Cookie{
		Name:     AUTH_TOKEN_JWT_COOKIE_NAME,
		Value:    tokenString,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Lax",
		MaxAge:   HOUR_IN_SECONDS * 4,
	}, nil
}

func VerifyJWTCookie(cookie string) (*User, error) {
	token, err := jwt.ParseWithClaims(cookie, &UserWithClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseRSAPublicKeyFromPEM([]byte(os.Getenv("JWT_PUBLIC_KEY")))
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*UserWithClaims)
	if !ok {
		return nil, err
	}

	return &claims.User, nil
}
