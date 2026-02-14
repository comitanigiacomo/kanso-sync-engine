package http

import (
	"errors"
	"net/http"

	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/domain"
	"github.com/comitanigiacomo/kanso-sync-engine/internal/core/services"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service *services.AuthService
}

func NewAuthHandler(service *services.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// Register godoc
// @Summary      Register a new user
// @Description  Create a new user account with email and password
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body registerRequest true "Registration Data"
// @Success      201  {object}  userResponse
// @Failure      400  {object}  map[string]string "Invalid Input / Password too short"
// @Failure      409  {object}  map[string]string "Email already exists"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.RegisterInput{
		Email:    req.Email,
		Password: req.Password,
	}

	user, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": "email already exists"})
		case errors.Is(err, domain.ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		case errors.Is(err, domain.ErrPasswordTooShort):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password too short"})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, userResponse{
		ID:    user.ID,
		Email: user.Email,
	})
}

// Login godoc
// @Summary      User Login
// @Description  Authenticates a user and returns a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body loginRequest true "Login Credentials"
// @Success      200  {object}  loginResponse
// @Failure      400  {object}  map[string]string "Invalid Input"
// @Failure      401  {object}  map[string]string "Invalid Credentials"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.LoginInput{
		Email:    req.Email,
		Password: req.Password,
	}

	output, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		default:
			_ = c.Error(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, loginResponse{
		Token: output.Token,
		User: struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		}{
			ID:    output.User.ID,
			Email: output.User.Email,
		},
	})
}

// Validate godoc
// @Summary      Validate Session
// @Description  Check if the current session/user is still valid
// @Tags         Auth
// @Security     BearerAuth
// @Success      200  {object}  map[string]string "Valid"
// @Failure      401  {object}  map[string]string "Unauthorized"
// @Router       /auth/validate [get]
func (h *AuthHandler) Validate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "valid"})
}

// DeleteAccount godoc
// @Summary      Delete User Account
// @Description  Permanently deletes the user and all associated data (habits, entries)
// @Tags         Auth
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]string "Account deleted"
// @Failure      401  {object}  map[string]string "Unauthorized"
// @Failure      500  {object}  map[string]string "Internal Server Error"
// @Router       /auth/user [delete]
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	if err := h.service.DeleteAccount(c.Request.Context(), userID.(string)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete account"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account deleted successfully"})
}

func (h *AuthHandler) RegisterRoutes(router *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)

		authGroup.GET("/validate", authMiddleware, h.Validate)
		authGroup.DELETE("/user", authMiddleware, h.DeleteAccount)
	}
}
