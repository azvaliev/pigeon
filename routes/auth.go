package routes

import (
	"fmt"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/azvaliev/pigeon/v2/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func AuthRoutes(router fiber.Router, db *sqlx.DB) {
	// OAuth2 Configuration
	githubOAuthConfig := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes: []string{
			"read:user",
			"read:email",
		},
		Endpoint: github.Endpoint,
	}

	router.Get("/", func(c *fiber.Ctx) error {
		return c.Render("auth", fiber.Map{})
	})

	type RegisterFormParams struct {
		Email  string `validate:"required,email"`
		Name   string
		Avatar string `validate:"required"`
	}

	router.Get("/register", func(c *fiber.Ctx) error {
		registerParams := &RegisterFormParams{
			Email:  c.Query("email"),
			Name:   c.Query("name"),
			Avatar: c.Query("avatar"),
		}

		if err := utils.ValidateStruct(*registerParams); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(err)
		}

		return c.Render("register", registerParams)
	})

	router.Get("/github", func(c *fiber.Ctx) error {
		// State state and user id used for signup
		state := ulid.Make()
		c.Cookie(&fiber.Cookie{
			Name:     "state",
			Value:    state.String(),
			Path:     "/",
			HTTPOnly: true,
			Secure:   true,
			SameSite: "Lax",
			MaxAge:   600,
		})

		c.Status(fiber.StatusSeeOther)
		c.Response().Header.Add("Location", githubOAuthConfig.AuthCodeURL(state.String()))
		return c.Send(nil)
	})

	type GithubCallbackParams struct {
		code  string `validate:"required"`
		state string `validate:"required,len=26"`
	}

	router.Get("/github/callback", func(c *fiber.Ctx) error {
		// Grab token and state from query, and validate them
		githubCallbackParams := &GithubCallbackParams{
			code:  c.Query("code"),
			state: c.Query("state"),
		}

		if err := utils.ValidateStruct(*githubCallbackParams); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(err)
		}

		// Get Github User Info
		userData, err := utils.GetGithubUserInfo(githubCallbackParams.code, githubOAuthConfig)
		if err != nil {
			return c.Status(fiber.StatusBadGateway).Send([]byte(err.Error()))
		}

		if userData.Email == "" {
			return c.Status(fiber.StatusNotFound).Send([]byte("Failed to find email for user"))
		}

		// Try to find corresponding user in DB
		user, err := utils.GetExistingUser(db, userData.Email)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
		}

		// If user exists, sign & send JWT, redirect to homepage
		if user != nil {
			jwtCookie, err := utils.CreateJWTCookieForUser(user)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
			}

			c.Cookie(jwtCookie)
			c.Status(fiber.StatusSeeOther)
			c.Response().Header.Add("Location", "/")
			return c.Send(nil)
		}

		// Create temporary registering token & send user to register
		jwtCookie, err := CreateRegisteringJWT(userData)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
		}

		c.Cookie(jwtCookie)
		c.Status(fiber.StatusSeeOther)
		c.Response().Header.Add(
			"Location",
			fmt.Sprintf(
				"/auth/register?email=%s&name=%s&avatar=%s",
				userData.Email,
				userData.Name,
				userData.AvatarUrl,
			),
		)
		return c.Send(nil)
	})

	type RegisterData struct {
		Email       string                `validate:"required,email,min=5,max=320" form:"email"`
		Username    string                `validate:"required,min=3,max=320" form:"username"`
		DisplayName string                `validate:"required,min=1,max=320" form:"display_name"`
		Avatar      *multipart.FileHeader `form:"avatar"`
	}

	router.Post("/register", func(c *fiber.Ctx) error {
		c.Accepts("multipart/form-data")

		// create register data from form data
		registerData := new(RegisterData)
		if err := c.BodyParser(registerData); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(err)
		}

		registerDataJwtCookie := c.Cookies(REGISTERING_JWT_COOKIE_NAME)
		if registerDataJwtCookie == "" {
			c.Status(fiber.StatusSeeOther)
			c.Response().Header.Add("Location", "/auth")
			return c.Send(nil)
		}

		// Decode JWT and make sure it is matching with form data
		jwtData, err := DecodeRegisteringJWT(registerDataJwtCookie)

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
		}
		if jwtData.Email != registerData.Email {
			return c.Status(fiber.StatusUnauthorized).Send([]byte("Email in JWT does not match email in form data"))
		}

		userId := ulid.Make().String()

		// Default to Github avatar if no avatar is provided
		avatarUrl := jwtData.AvatarUrl

		if registerData.Avatar != nil && registerData.Avatar.Filename != "" {
			// Get an S3 Connection
			uploader, err := utils.CreateS3Uploader()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
			}

			// Reading Avatar File
			avatar, err := registerData.Avatar.Open()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
			}

			// Upload Avatar to S3
			s3UploadResult, err := uploader.Upload(&s3manager.UploadInput{
				Bucket: aws.String(os.Getenv("AWS_S3_BUCKET")),
				Key:    aws.String(userId),
				Body:   avatar,
				ACL:    aws.String("public-read"),
			})

			if err != nil {
				return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
			}

			avatarUrl = s3UploadResult.Location
		}

		// Create user in DB
		_, err = db.Exec(
			"INSERT INTO User (id, email, username, display_name, avatar) VALUES (?, ?, ?, ?, ?)",
			userId,
			registerData.Email,
			registerData.Username,
			registerData.DisplayName,
			avatarUrl,
		)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
		}

		// Create JWT for user
		userJWT, err := utils.CreateJWTCookieForUser(&utils.User{
			Id:          userId,
			Email:       registerData.Email,
			DisplayName: registerData.DisplayName,
			Avatar:      avatarUrl,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).Send([]byte(err.Error()))
		}

		c.Cookie(userJWT)
		c.Status(fiber.StatusSeeOther)
		c.Response().Header.Add("Location", "/")
		return c.Send(nil)
	})
}

const REGISTERING_JWT_COOKIE_NAME = "registering-data"

type RegisteringJWTClaims struct {
	UserData utils.GithubUserData `json:"user_data"`
	jwt.RegisteredClaims
}

func CreateRegisteringJWT(githubUserData *utils.GithubUserData) (*fiber.Cookie, error) {
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
		MaxAge:   utils.HOUR_IN_SECONDS * (1 / 3),
	}, nil
}

func DecodeRegisteringJWT(cookie string) (*utils.GithubUserData, error) {
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
