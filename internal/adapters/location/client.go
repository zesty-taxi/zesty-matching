package location

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/zesty-taxi/zesty-matching/internal/domain"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:   5 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

type nearbyResponse struct {
	Drivers []domain.NearbyDriver `json:"drivers"`
}

// GetNearbyDrivers — получить ближайших водителей из Location Service.
// Возвращает список отсортированный по расстоянию (Location Service сортирует сам).
func (c *Client) GetNearbyDrivers(ctx context.Context, lat, lng float64, radiusM int) ([]domain.NearbyDriver, error) {
	url := fmt.Sprintf(
		"%s/api/v1/location/nearby?lat=%f&lng=%f&radius=%d",
		c.baseURL, lat, lng, radiusM,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("location client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("location client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("location client: unexpected status %d", resp.StatusCode)
	}

	var result nearbyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("location client: decode response: %w", err)
	}

	return result.Drivers, nil
}
