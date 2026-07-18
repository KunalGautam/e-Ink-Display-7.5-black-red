package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type WeatherData struct {
	Temp       float64        `json:"temp"`
	Condition  string         `json:"condition"`
	Humidity   int            `json:"humidity"`
	Pressure   float64        `json:"pressure"`
	WindSpeed  float64        `json:"wind_speed"`
	Forecast   []ForecastItem `json:"forecast"`
}

type ForecastItem struct {
	Time      string  `json:"time"`
	Temp      float64 `json:"temp"`
	Condition string  `json:"condition"`
}

type WeatherClient struct {
	mu           sync.RWMutex
	cachedData   *WeatherData
	lastUpdated  time.Time
}

func NewWeatherClient() *WeatherClient {
	return &WeatherClient{}
}

func (w *WeatherClient) GetWeather() *WeatherData {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cachedData
}

func (w *WeatherClient) SetWeather(data *WeatherData) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cachedData = data
	w.lastUpdated = time.Now()
}

func (w *WeatherClient) GetLastUpdated() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastUpdated
}

// FetchOpenWeatherMap fetches data from OpenWeatherMap API and updates the cache
func (w *WeatherClient) FetchOpenWeatherMap(apiKey, city string) error {
	if apiKey == "" || city == "" {
		return fmt.Errorf("API key or city is empty")
	}

	// 1. Fetch Current Weather
	currentURL := fmt.Sprintf("https://api.openweathermap.org/data/2.5/weather?q=%s&appid=%s&units=metric", url.QueryEscape(city), apiKey)
	resp, err := http.Get(currentURL)
	if err != nil {
		return fmt.Errorf("failed to fetch current weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("current weather API returned status code %d", resp.StatusCode)
	}

	var currentResult struct {
		Main struct {
			Temp     float64 `json:"temp"`
			Humidity int     `json:"humidity"`
			Pressure float64 `json:"pressure"`
		} `json:"main"`
		Weather []struct {
			Main string `json:"main"`
		} `json:"weather"`
		Wind struct {
			Speed float64 `json:"speed"`
		} `json:"wind"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&currentResult); err != nil {
		return fmt.Errorf("failed to decode current weather: %w", err)
	}

	// 2. Fetch Forecast
	forecastURL := fmt.Sprintf("https://api.openweathermap.org/data/2.5/forecast?q=%s&appid=%s&units=metric", url.QueryEscape(city), apiKey)
	respF, err := http.Get(forecastURL)
	if err != nil {
		return fmt.Errorf("failed to fetch forecast: %w", err)
	}
	defer respF.Body.Close()

	if respF.StatusCode != http.StatusOK {
		return fmt.Errorf("forecast API returned status code %d", respF.StatusCode)
	}

	var forecastResult struct {
		List []struct {
			Dt   int64 `json:"dt"`
			Main struct {
				Temp float64 `json:"temp"`
			} `json:"main"`
			Weather []struct {
				Main string `json:"main"`
			} `json:"weather"`
		} `json:"list"`
	}

	if err := json.NewDecoder(respF.Body).Decode(&forecastResult); err != nil {
		return fmt.Errorf("failed to decode forecast: %w", err)
	}

	// Process forecast items (extract first 4 intervals, each is 3 hours apart)
	var forecastItems []ForecastItem
	limit := len(forecastResult.List)
	if limit > 4 {
		limit = 4
	}
	for i := 0; i < limit; i++ {
		item := forecastResult.List[i]
		t := time.Unix(item.Dt, 0)
		
		condition := "Cloudy"
		if len(item.Weather) > 0 {
			condition = item.Weather[0].Main
		}
		
		forecastItems = append(forecastItems, ForecastItem{
			Time:      t.Format("3 PM"), // Format as "1 PM", "4 PM", etc.
			Temp:      item.Main.Temp,
			Condition: condition,
		})
	}

	// If no condition was returned in current weather, default to Cloudy
	currCondition := "Cloudy"
	if len(currentResult.Weather) > 0 {
		currCondition = currentResult.Weather[0].Main
	}

	data := &WeatherData{
		Temp:       currentResult.Main.Temp,
		Condition:  currCondition,
		Humidity:   currentResult.Main.Humidity,
		Pressure:   currentResult.Main.Pressure,
		WindSpeed:  currentResult.Wind.Speed,
		Forecast:   forecastItems,
	}

	w.SetWeather(data)
	return nil
}
