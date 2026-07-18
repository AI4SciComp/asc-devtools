package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListPaginationHeadersAndFiltering(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		if request.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if request.Header.Get("X-GitHub-Api-Version") != "2022-11-28" || request.Header.Get("User-Agent") != "asc-devtools/0.1.0" {
			t.Errorf("missing required headers: %v", request.Header)
		}
		if request.URL.Query().Get("page") == "1" {
			writer.Header().Set("Link", fmt.Sprintf(`<%s?page=2>; rel="next"`, serverURL(request)))
			fmt.Fprint(writer, `[{"name":"asc-z","archived":false,"ssh_url":"ssh-z","clone_url":"https-z"},{"name":"other","archived":false}]`)
			return
		}
		fmt.Fprint(writer, `[{"name":".github","archived":false},{"name":"asc-old","archived":true},{"name":"asc-a","archived":false,"private":true}]`)
	}))
	defer server.Close()
	client := NewClient("test-secret", "0.1.0")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	repositories, err := client.ListOrganizationRepositories(context.Background(), "AI4SciComp")
	if err != nil {
		t.Fatal(err)
	}
	filtered := FilterManaged(repositories, "asc-", true)
	if requests != 2 || len(filtered) != 3 || filtered[0].Name != ".github" || filtered[2].Name != "asc-z" {
		t.Fatalf("requests=%d filtered=%+v", requests, filtered)
	}
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}

func TestListWithoutTokenOmitsAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			t.Errorf("unexpected Authorization header")
		}
		fmt.Fprint(writer, `[]`)
	}))
	defer server.Close()
	client := NewClient("", "dev")
	client.BaseURL = server.URL
	client.HTTPClient = server.Client()
	if _, err := client.ListOrganizationRepositories(context.Background(), "AI4SciComp"); err != nil {
		t.Fatal(err)
	}
}

func TestListAPIErrorMessagesDoNotLeakToken(t *testing.T) {
	for _, test := range []struct {
		status  int
		headers map[string]string
		want    string
	}{
		{http.StatusUnauthorized, nil, "authentication failed"},
		{http.StatusForbidden, map[string]string{"X-RateLimit-Remaining": "0"}, "rate limit"},
		{http.StatusForbidden, nil, "forbidden"},
		{http.StatusNotFound, nil, "not found"},
	} {
		t.Run(fmt.Sprint(test.status, test.want), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				for name, value := range test.headers {
					writer.Header().Set(name, value)
				}
				writer.WriteHeader(test.status)
				fmt.Fprint(writer, `{"message":"secret-token"}`)
			}))
			defer server.Close()
			client := NewClient("secret-token", "dev")
			client.BaseURL = server.URL
			client.HTTPClient = server.Client()
			_, err := client.ListOrganizationRepositories(context.Background(), "AI4SciComp")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestListMalformedAndOversizedResponses(t *testing.T) {
	for _, body := range []string{`not-json`, strings.Repeat("x", maxResponseSize+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(writer, body)
		}))
		client := NewClient("", "dev")
		client.BaseURL = server.URL
		client.HTTPClient = server.Client()
		_, err := client.ListOrganizationRepositories(context.Background(), "AI4SciComp")
		server.Close()
		if err == nil {
			t.Fatal("ListOrganizationRepositories() succeeded for invalid response")
		}
	}
}

func TestListTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		fmt.Fprint(writer, `[]`)
	}))
	defer server.Close()
	client := NewClient("", "dev")
	client.BaseURL = server.URL
	client.HTTPClient = &http.Client{Timeout: 10 * time.Millisecond}
	_, err := client.ListOrganizationRepositories(context.Background(), "AI4SciComp")
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("error = %v", err)
	}
}
