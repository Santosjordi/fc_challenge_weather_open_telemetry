package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"service-b/config"
	"strings"
	"testing"
)

func TestHandler_Success(t *testing.T) {
	// Mock external APIs
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws/") { // Check for ViaCEP path prefix
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"localidade": "São Paulo"}`))
		} else if strings.HasPrefix(r.URL.Path, "/weatherapi") { // Check for WeatherAPI path
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current": {"temp_c": 25.0}}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Override URLs to point to the mock server
	originalViaCepURL := viaCepURL
	viaCepURL = mockServer.URL + "/ws/%s/json" // The test mock doesn't need the final /
	defer func() {
		viaCepURL = originalViaCepURL
	}()

	// Setup app with mock config
	app := &app{
		cfg: &config.Config{WeatherAPIKey: "test-key"},
	}

	// Override the getWeatherURL function to point to our mock server
	originalGetWeatherURL := getWeatherURL
	getWeatherURL = func(apiKey, city string) string { return mockServer.URL + "/weatherapi" }
	defer func() { getWeatherURL = originalGetWeatherURL }()

	body := `{"cep":"01001000"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	app.handler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := TempResponse{
		City:  "São Paulo",
		TempC: 25.0,
		TempF: 77.0,
		TempK: 298.0,
	}
	var actual TempResponse
	if err := json.NewDecoder(rr.Body).Decode(&actual); err != nil {
		t.Fatalf("Could not decode response: %v", err)
	}

	if actual != expected {
		t.Errorf("handler returned unexpected body: got %+v want %+v", actual, expected)
	}
}

func TestHandler_InvalidZipcode(t *testing.T) {
	app := &app{} // No config needed for this test

	body := `{"cep":"12345"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	app.handler(rr, req)

	if status := rr.Code; status != http.StatusUnprocessableEntity {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnprocessableEntity)
	}

	expected := "invalid zipcode\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q", rr.Body.String(), expected)
	}
}

func TestHandler_ZipcodeNotFound(t *testing.T) {
	// Mock ViaCEP to return a "not found" response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ws/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"erro": true}`))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	// Override URL to point to the mock server
	originalViaCepURL := viaCepURL
	viaCepURL = mockServer.URL + "/ws/%s/json"
	defer func() { viaCepURL = originalViaCepURL }()

	// Setup app with mock config
	app := &app{
		cfg: &config.Config{WeatherAPIKey: "test-key"},
	}

	// Use the CEP from the original curl command that caused the error
	// This is a valid format but doesn't exist in the ViaCEP database.
	body := `{"cep":"01001003"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	app.handler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}

	expected := "can not find zipcode\n"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %q want %q", rr.Body.String(), expected)
	}
}
