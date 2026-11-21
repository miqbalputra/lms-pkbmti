package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type reportTestTransport func(*http.Request) (*http.Response, error)

func (f reportTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestReportAIProviders(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-openai")
	t.Setenv("GEMINI_API_KEY", "test-gemini")
	for _, tc := range []struct {
		name, provider, body string
		status               int
		wantError            bool
	}{
		{"openai", "openai", `{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"strength\":\"Mampu membaca.\",\"growth\":\"Latihan menulis.\"}"}]}]}`, 200, false},
		{"gemini", "gemini", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"{\"strength\":\"Mampu membaca.\",\"growth\":\"Latihan menulis.\"}"}]}}]}`, 200, false},
		{"incomplete", "openai", `{"status":"incomplete"}`, 200, true},
		{"refusal", "openai", `{"status":"completed","output":[{"content":[{"type":"refusal"}]}]}`, 200, true},
		{"quota", "gemini", `{}`, 429, true},
		{"blocked", "gemini", `{"candidates":[{"finishReason":"SAFETY"}]}`, 200, true},
		{"wrong schema", "openai", `{"status":"completed","output":[{"content":[{"type":"output_text","text":"{\"strength\":12}"}]}]}`, 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{Transport: reportTestTransport(func(r *http.Request) (*http.Response, error) {
				data, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(data), "strength") {
					t.Fatal("structured schema missing")
				}
				if tc.provider == "openai" && r.Header.Get("Authorization") != "Bearer test-openai" {
					t.Fatal("wrong bearer")
				}
				if tc.provider == "gemini" && r.Header.Get("x-goog-api-key") != "test-gemini" {
					t.Fatal("wrong gemini key")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}
			result, err := reportGenerateAIClient(context.Background(), ReportSettings{Provider: tc.provider, Model: "test-model"}, "Academic evidence", client)
			if (err != nil) != tc.wantError {
				t.Fatalf("result %+v err %v", result, err)
			}
			if !tc.wantError && result.Strength != "Mampu membaca." {
				t.Fatal(result)
			}
		})
	}
}
