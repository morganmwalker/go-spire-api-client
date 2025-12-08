package spireclient

import (
	"encoding/json"
	"encoding/base64"
	"io"
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// API client configuration
type SpireClient struct {
    RootURL string
    HTTPClient *http.Client
}

// SpireAgent holds the authentication details (must be passed in every request)
type SpireAgent struct {
    Username string
    Password string
}

// SpireClient constructor
func NewSpireClient(rootURL string) *SpireClient {
    return &SpireClient{
        RootURL: rootURL,
        HTTPClient: &http.Client{
            Timeout: 10 * time.Second, 
        },
    }
}

// Generates the basic authentication headers required by Spire
func (a SpireAgent) BasicAuthHeader() string {
    encodedCredentials := base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
    return "Basic " + encodedCredentials
}

type SpireError struct {
    Status string
    Detail string
}

func (e *SpireError) Error() string {
    return fmt.Sprintf("API request failed with status %s. Details: %s", e.Status, e.Detail)
}

type SpireResponse struct {
    Records []map[string]interface{} `json:"records"`
    Count   float64                  `json:"count"`
}

// Performs an HTTP request to the Spire server handles payload marshaling, and authentication
// Expects a SpireResponse body on success (200 OK) or an empty body on creation/deletion (201, 204)
func (c *SpireClient) SpireRequest(fullURL string, agent SpireAgent, method string, payload interface{}) (SpireResponse, error) { 
    var bodyReader io.Reader
    if payload != nil {
        payloadBytes, err := json.Marshal(payload)
        if err != nil {
            return SpireResponse{}, fmt.Errorf("failed to marshal payload: %w", err)
        }
        bodyReader = bytes.NewReader(payloadBytes)
    }

    req, err := http.NewRequest(method, fullURL, bodyReader)
    if err != nil {
        return SpireResponse{}, fmt.Errorf("error creating request: %w", err)
    }

    if payload != nil {
        req.Header.Set("Content-Type", "application/json")
    }

    encodedCredentials := base64.StdEncoding.EncodeToString([]byte(agent.Username + ":" + agent.Password))
    req.Header.Set("Authorization", "Basic " + encodedCredentials)
    
    resp, err := c.HTTPClient.Do(req)
    
    if err != nil {
        return SpireResponse{}, fmt.Errorf("error making request to %s: %w", fullURL, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
        responseBody, readErr := io.ReadAll(resp.Body)
        if readErr != nil {
            return SpireResponse{}, fmt.Errorf("request failed with status %s, but failed to read error body: %w", resp.Status, readErr)
        }
        apiErrorMessage := string(responseBody)
        return SpireResponse{}, fmt.Errorf("API request failed with status %s. Details: %s", resp.Status, apiErrorMessage)  
    }
    
    if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
        return SpireResponse{}, nil
    }
    
    var spireResponse SpireResponse 

    if err := json.NewDecoder(resp.Body).Decode(&spireResponse); err != nil {
        return SpireResponse{}, fmt.Errorf("error unmarshaling JSON: %w", err)
    }

    return spireResponse, nil
}

// Attempts to get rool url to check if provided credentials are valid
func (c *SpireClient) ValidateSpireCredentials(agent SpireAgent) error {
    reqURL := c.RootURL
    
    req, err := http.NewRequest("GET", reqURL, nil)
    if err != nil {
        return fmt.Errorf("error creating validation request: %w", err)
    }
    
    req.Header.Set("Authorization", agent.BasicAuthHeader()) 
    
    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return fmt.Errorf("error calling Spire validation API: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body) 
        return &SpireError{
            Status: resp.Status,
            Detail: string(body),
        }
    }
    return nil
}

// Converts maps to JSON string
func ConvertFilter(filters map[string]interface{}) (string, error) {
    if filters == nil || len(filters) == 0 {
        return "", nil
    }

    jsonBytes, err := json.Marshal(filters)
    if err != nil {
        return "", fmt.Errorf("failed to marshal filter to JSON: %w", err)
    }
	
    return string(jsonBytes), nil
}

// Gets ALL records for a given endpoint
func (c *SpireClient) FetchSpireData(endpoint string, filters map[string]interface{}, agent SpireAgent) ([]map[string]interface{}, error) {
	maxLimit := 1000

	filter, err := ConvertFilter(filters)
    if err != nil {
        return nil, fmt.Errorf("could not convert filter: %w", err)
    }
	
	baseURL, err := url.Parse(endpoint)
    if err != nil {
        return nil, fmt.Errorf("invalid endpoint URL: %w", err)
    }

	q := baseURL.Query()
    q.Set("limit", fmt.Sprintf("%d", maxLimit))
    if filter != "" {
        q.Set("filter", filter)
    }
	baseURL.RawQuery = q.Encode()
	
	initialResponse, err := c.SpireRequest(baseURL.String(), agent, "GET", nil)
    if err != nil {
        return nil, fmt.Errorf("error making initial Spire request: %w", err)
    }

	records := initialResponse.Records
    count := initialResponse.Count
	remainingRequests := (int(count) + maxLimit - 1) / maxLimit - 1

	for i := 1; i < remainingRequests; i++ {
		start := maxLimit * i

		q.Set("start", fmt.Sprintf("%d", start))
		baseURL.RawQuery = q.Encode()

		nextPageResponse, err := c.SpireRequest(baseURL.String(), agent, "GET", nil)
		if err != nil {
			return nil, fmt.Errorf("error making Spire request for page %d: %w", i+2, err)
		}
		records = append(records, nextPageResponse.Records...)
	}

	return records, nil
}

type OrderDetails struct {
    OrderNo string `json:"orderNo"`
    PurchaseNo string `json:"purchaseNo"`
}

// Gets all sales items associated with the provided map of orders
func(c *SpireClient) GetOrderItems(orders map[string]OrderDetails, agent SpireAgent) ([]map[string]interface{}, error) {
    // Make a filter for an HTTP request that gets the items for every order submitted
    // Should look like:
    // { "$or": [ { "orderNo": orderNo1 }, { "orderNo": orderNo2}, ... ] }
    noOrders := len(orders)

    orConditions := make([]map[string]string, 0, noOrders)

    for _, order := range orders {
        condition := map[string]string{"orderNo": order.OrderNo}
        orConditions = append(orConditions, condition)
    }

    itemFilter := map[string]interface{}{"$or": orConditions}

    items, err := c.FetchSpireData(c.RootURL+"/sales/items", itemFilter, agent)
    if err != nil {
        return nil, err
    }
    return items, nil
}

// Sends a POST request to Spire to create a new sales order
// The payload should be the fully prepared sales order body structure
func (c *SpireClient) CreateSalesOrder(agent SpireAgent, payload interface{}) (SpireResponse, error) {
    // Implementation for the missing function:
    return c.SpireRequest(c.RootURL+"/sales/orders", agent, "POST", payload)
}

// Loops through a list of sales order IDs and tries to delete the orders in Spire
func(c *SpireClient) DeleteSalesOrders(orderList []string, agent SpireAgent) error {
    for _, orderID := range orderList {
        _, err := c.SpireRequest(c.RootURL+"/sales/orders/"+orderID, agent, "DELETE", nil) 
        if err != nil {
            return fmt.Errorf("failed to delete order %s: %w", orderID, err)
        }
    }
    return nil

}
