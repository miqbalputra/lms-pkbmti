package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestCBTSSOTicketIsShortLivedAndBoundToAuthenticatedLMSUser(t *testing.T) {
	s := testServer(t)
	t.Setenv("CBT_SSO_HMAC_SECRET", "independent-test-cbt-sso-secret-32-chars")
	user := User{Username: "sso-guru", Role: "guru", IsActive: true}
	if err := s.db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	accessToken, err := s.token(user, s.cfg.AccessSecret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/ticket", s.auth, s.issueCBTSSOTicket)
	body := `{"state":"` + uuid.NewString() + `"}`
	request := httptest.NewRequest(http.MethodPost, "/ticket", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected SSO ticket, got HTTP %d", response.StatusCode)
	}
	var result struct {
		Ticket string    `json:"ticket"`
		Expiry time.Time `json:"expiresAt"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Ticket == "" || time.Until(result.Expiry) > 2*time.Minute || time.Until(result.Expiry) < time.Minute {
		t.Fatalf("unexpected ticket expiry: %+v", result)
	}
	claims := &cbtSSOClaims{}
	token, err := jwt.ParseWithClaims(result.Ticket, claims, func(token *jwt.Token) (any, error) {
		return []byte("independent-test-cbt-sso-secret-32-chars"), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(cbtSSOIssuer), jwt.WithAudience(cbtSSOAudience))
	if err != nil || token == nil || !token.Valid {
		t.Fatalf("signed ticket did not validate: token=%v err=%v", token, err)
	}
	if claims.Subject != user.ID || claims.Username != user.Username || claims.Role != "guru" || claims.State == "" {
		t.Fatalf("ticket claims do not match the authenticated LMS user: %+v", claims)
	}
}

func TestCBTSSOTicketRejectsParentAndMissingSecret(t *testing.T) {
	s := testServer(t)
	parent := User{Username: "sso-parent", Role: "orang_tua", IsActive: true}
	if err := s.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	accessToken, err := s.token(parent, s.cfg.AccessSecret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/ticket", s.auth, s.issueCBTSSOTicket)
	t.Setenv("CBT_SSO_HMAC_SECRET", "")
	request := httptest.NewRequest(http.MethodPost, "/ticket", strings.NewReader(`{"state":"`+uuid.NewString()+`"}`))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected fail-closed response without SSO secret, got HTTP %d", response.StatusCode)
	}
	t.Setenv("CBT_SSO_HMAC_SECRET", "independent-test-cbt-sso-secret-32-chars")
	request = httptest.NewRequest(http.MethodPost, "/ticket", strings.NewReader(`{"state":"`+uuid.NewString()+`"}`))
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("parent must not receive CBT ticket, got HTTP %d", response.StatusCode)
	}
}
