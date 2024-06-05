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

	ctx2, span := tracer.Start(ctx, "service_a")
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

	ctx3, viaCepSpan := tracer.Start(ctx2, "service_a: via-cep-request")

	// Make the request to the ViaCep API
	viaCepResponse, err := http.DefaultClient.Do(req)

	// End the ViaCep request trace
	viaCepSpan.End()

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
	ctx4, serviceBSpan := tracer.Start(ctx3, "service_a: get-from-service-b")

	// Create a new request to the temperature microservice and propagate the tracing context
	tempURL := "http://service-b:8181/" + city
	tempReq, err := http.NewRequest(http.MethodGet, tempURL, nil)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	otel.GetTextMapPropagator().Inject(ctx4, propagation.HeaderCarrier(tempReq.Header))

	// Make the request to the temperature microservice
	tempResponse, err := http.DefaultClient.Do(tempReq)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	defer tempResponse.Body.Close()

	// Deserialize the JSON response from the temperature microservice
	var temperatureData TemperatureData
	err = json.NewDecoder(tempResponse.Body).Decode(&temperatureData)
	if err != nil {
		HandleError(w, http.StatusInternalServerError, "Failed to decode temperature data", err)
		return
	}

	// Set response header and send the temperature data
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(temperatureData)

	serviceBSpan.End()
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
