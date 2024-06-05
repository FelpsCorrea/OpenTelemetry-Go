package handlers

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/go-chi/chi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// CelsiusToFahrenheit converts a temperature from Celsius to Fahrenheit
func CelsiusToFahrenheit(celsius float64) float64 {
	return celsius*1.8 + 32
}

// CelsiusToKelvin converts a temperature from Celsius to Kelvin
func CelsiusToKelvin(celsius float64) float64 {
	return celsius + 273.15
}

// GetTemperature handles the request to get the current temperature for a given city
func GetTemperature(w http.ResponseWriter, r *http.Request) {
	// Extract tracing context from request headers
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), carrier)
	tracer := otel.Tracer("weatherapi")

	// Start tracing the temperature API request
	ctx2, span := tracer.Start(ctx, "service_b")
	defer span.End()

	// Get the city parameter from the URL
	city := chi.URLParam(r, "city")
	encodedCity := url.QueryEscape(city)

	// Set up the HTTP client with TLS configuration
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// Build the request URL for the weather API
	apiURL := "https://api.weatherapi.com/v1/current.json?q=" + encodedCity + "&key=360ddfd38d0d4cd3b72102808240403"
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Start tracing the temperature request
	_, calculateTempSpan := tracer.Start(ctx2, "service_b: weather-api-request")

	// Make the request to the weather API
	resp, err := client.Do(req)

	// End the temperature request trace
	calculateTempSpan.End()
	if err != nil {
		http.Error(w, "Failed to fetch weather data", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Weather service returned non-OK status", resp.StatusCode)
		return
	}

	// Decode the JSON response from the weather API
	var data WeatherResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		http.Error(w, "Failed to decode weather data", http.StatusInternalServerError)
		return
	}

	// Calculate temperatures in different scales
	result := &Temperature{
		Celsius:    data.Current.TemperatureC,
		Fahrenheit: CelsiusToFahrenheit(data.Current.TemperatureC),
		Kelvin:     CelsiusToKelvin(data.Current.TemperatureC),
	}

	// Set response header and send the temperature data as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"city":       city,
		"celsius":    result.Celsius,
		"fahrenheit": result.Fahrenheit,
		"kelvin":     result.Kelvin,
	}
	json.NewEncoder(w).Encode(response)

}

// Structs for parsing the weather API response and formatting the temperature data
type WeatherResponse struct {
	Current struct {
		TemperatureC float64 `json:"temp_c"`
	} `json:"current"`
}

type Temperature struct {
	Celsius    float64 `json:"celsius"`
	Fahrenheit float64 `json:"fahrenheit"`
	Kelvin     float64 `json:"kelvin"`
}
