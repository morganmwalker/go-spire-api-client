# go-spire-api-client

A **Go package** for interaction with the **Spire Systems API**.

This client handles basic authentication, JSON marshaling, and automatic pagination for large data fetches.

## Installation

To install the library, use `go get`:

```bash
go get github.com/morganmwalker/go-spire-api-client
```

## Usage

### Initialization and Authentication

Create a SpireClient and provide your Spire URL in the form `https://{spire-url}:10880/api/v2/companies/{company}`. Authentication credentials are held in the SpireAgent struct and must be passed with every API call.

```go
package main

import (
	"fmt"
	"os"
	"github.com/morganmwalker/go-spire-api-client"
)

func main() {
    spireURL := os.Getenv("SPIRE_URL")
    username := os.Getenv("SPIRE_USERNAME")
    password := os.Getenv("SPIRE_PASSWORD")

    client := spireclient.NewSpireClient(spireURL)

    agent := spireclient.SpireAgent{ 
        Username: username,
        Password: password,
    }

    if err := client.ValidateSpireCredentials(agent); err != nil {
		fmt.Printf("Credential validation failed: %v\n", err)
		return
	}
	fmt.Println("Successfully authenticated with Spire API.")
}
```

### Data Fetching
Use the `FetchSpireData` method to retrieve all records for a given endpoint. This method automatically handles the API's pagination (using the limit and start parameters) to fetch all available records.

```Go
// Define filters as a Go map
salesOrderFilter := map[string]interface{}{
    "type": "O"
}

salesOrders, err := client.FetchSpireData("/sales/orders", salesOrderFilter, agent)
if err != nil {
	fmt.Printf("Error fetching sales orders: %v\n", err)
	return
}

fmt.Printf("Successfully fetched %d sales orders.\n", len(openOrders))
```

### Creating Records (POST)
Use the specific creation methods, or SpireRequest directly. The payload must be a Go struct or map that matches the expected JSON structure for the Spire endpoint.
```Go
// Using CreateSalesOrder
response, err := client.CreateSalesOrder(agent, submitPayload)

// Using SpireRequest
response, err := client.SpireRequest("/sales/orders", agent, "POST", submitPayload)
```

### Error Handling
The client uses standard Go error patterns and includes a custom `SpireError` struct for HTTP status codes that indicate an API failure (non-200/201/204).

When an API request returns an error status (e.g., 400 Bad Request, 401 Unauthorized), the `SpireRequest` or `ValidateSpireCredentials` method will return an error that includes the HTTP status and the raw response body from the API, if available.
