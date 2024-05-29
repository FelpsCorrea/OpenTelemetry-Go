package handlers

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/go-chi/chi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

type ViaCep struct {
	Cep        string `json:"cep"`
	Localidade string `json:"localidade"`
	Error      bool   `json:"erro"`
}

// TemperatureData represents the structure of the temperature data
type TemperatureData struct {
	City       string  `json:"city"`
	Celsius    float64 `json:"celsius"`
	Fahrenheit float64 `json:"fahrenheit"`
	Kelvin     float64 `json:"kelvin"`
}

// HTTPClient is an interface for making HTTP requests, useful for testing
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// validateCEP checks if the provided CEP is in the correct format
func validateCEP(cep string) bool {
	regex := regexp.MustCompile(`^\d{8}$|^\d{5}-\d{3}$`)
	return regex.MatchString(cep)
}

// GetTemperature handles the request to get the temperature of a city based on a CEP
func GetWeatherHandler(w http.ResponseWriter, r *http.Request) {
	// Extract tracing context
	carrier := propagation.HeaderCarrier(r.Header)
	ctx := otel.GetTextMapPropagator().Extract(r.Context(), carrier)
	tracer := otel.Tracer("viacepapi")

	// Start tracing the request to the ViaCep API
	ctx, span := tracer.Start(ctx, "send-viacep")
	defer span.End()

	cep := chi.URLParam(r, "cep")
	if !validateCEP(cep) {
		HandleError(w, http.StatusNotFound, "invalid zipcode", nil)
		return
	}

	// Build the request URL for the ViaCep API
	url1 := "http://viacep.com.br/ws/" + cep + "/json/"
	req, err := http.NewRequest(http.MethodGet, url1, nil)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// Make the request to the ViaCep API
	viaCepResponse, err := http.DefaultClient.Do(req)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer viaCepResponse.Body.Close()

	// Read the response body from the ViaCep API
	body, err := io.ReadAll(viaCepResponse.Body)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// Deserialize the JSON response from the ViaCep API
	var data ViaCep
	err = json.Unmarshal(body, &data)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	if data.Error {
		HandleError(w, http.StatusNotFound, "cannot find zipcode", nil)
		return
	}

	city := data.Localidade

	// Start tracing the request to the temperature microservice
	_, renderContent := tracer.Start(ctx, "get-from-microservice-b")

	// Make the request to the temperature microservice
	temperatura, err := http.Get("http://service-b:8181/" + city)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer temperatura.Body.Close()

	// Deserialize the JSON response from the temperature microservice
	var temperatureData TemperatureData
	err = json.NewDecoder(temperatura.Body).Decode(&temperatureData)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, "Failed to decode temperature data", err)
		return
	}

	// Set response header and send the temperature data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(temperatureData)
	renderContent.End()
}

// HandleError handles errors and sends a JSON response with the error message
func HandleError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResponse := map[string]string{"error": message}
	json.NewEncoder(w).Encode(errorResponse)
	if err != nil {
		log.Println(err)
	}
}
