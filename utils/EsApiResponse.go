package utils

import (
	"fmt"
	"io"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func ReadResponseBody(response *esapi.Response) (string, error) {
	if response.Body == nil {
		return "", fmt.Errorf("response body is nil")
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
