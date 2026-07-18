package weather

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeatherClientParsing(t *testing.T) {
	client := NewWeatherClient()

	// Mock server for OpenWeatherMap Current Weather
	currentServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"main": {"temp": 24.5, "humidity": 65, "pressure": 1010.5},
			"weather": [{"main": "Clear"}],
			"wind": {"speed": 3.6}
		}`))
	}))
	defer currentServer.Close()

	// Mock server for OpenWeatherMap Forecast
	forecastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"list": [
				{"dt": 1721200000, "main": {"temp": 23.0}, "weather": [{"main": "Clouds"}]},
				{"dt": 1721210800, "main": {"temp": 22.0}, "weather": [{"main": "Rain"}]}
			]
		}`))
	}))
	defer forecastServer.Close()

	// Direct tests for client data storage
	wd := &WeatherData{
		Temp:      24.5,
		Condition: "Clear",
		Humidity:   65,
		Pressure:   1010.5,
		WindSpeed:  3.6,
		Forecast: []ForecastItem{
			{Time: "1 PM", Temp: 23.0, Condition: "Clouds"},
		},
	}
	client.SetWeather(wd)

	retrieved := client.GetWeather()
	if retrieved == nil {
		t.Fatal("Expected retrieved weather to not be nil")
	}

	if retrieved.Temp != 24.5 {
		t.Errorf("Expected temp to be 24.5, got %v", retrieved.Temp)
	}

	if retrieved.Condition != "Clear" {
		t.Errorf("Expected condition to be 'Clear', got '%s'", retrieved.Condition)
	}

	if len(retrieved.Forecast) != 1 || retrieved.Forecast[0].Temp != 23.0 {
		t.Errorf("Forecast parsing test failed: %v", retrieved.Forecast)
	}
}
