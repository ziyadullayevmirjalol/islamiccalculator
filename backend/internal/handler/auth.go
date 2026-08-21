package handler

import (
	"context"
	"net/http"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

type AuthService interface {
	Register(ctx context.Context, email, password string) (service.AuthOutput, error)
	Login(ctx context.Context, email, password string) (service.AuthOutput, error)
	Refresh(ctx context.Context, refreshToken string) (service.AuthOutput, error)
}

type Auth struct {
	svc AuthService
}

func NewAuth(svc AuthService) *Auth {
	return &Auth{svc: svc}
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type authResponse struct {
	User struct {
		ID    string `json:"id"`
		Email string `json:"email,omitempty"`
	} `json:"user"`
	AccessToken     string `json:"accessToken"`
	RefreshToken    string `json:"refreshToken"`
	AccessExpiresIn int64  `json:"accessExpiresInSeconds"`
}

func toAuthResponse(out service.AuthOutput) authResponse {
	var resp authResponse
	resp.User.ID = out.UserID
	resp.User.Email = out.Email
	resp.AccessToken = out.AccessToken
	resp.RefreshToken = out.RefreshToken
	resp.AccessExpiresIn = out.AccessExpiresIn
	return resp
}

func (h *Auth) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}
	out, err := h.svc.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.Err(w, err)
		return
	}
	httpx.Data(w, http.StatusCreated, toAuthResponse(out))
}

func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}
	out, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.Err(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, toAuthResponse(out))
}

func (h *Auth) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}
	if req.RefreshToken == "" {
		httpx.Err(w, apperr.Validation("refreshToken is required",
			map[string]string{"refreshToken": "required"}))
		return
	}
	out, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.Err(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, toAuthResponse(out))
}
