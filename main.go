package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

var loadedSpec *openapi3.T // Store the loaded OpenAPI spec

func main() {
	r := gin.Default()
	r.SetTrustedProxies([]string{"192.168.0.0/16", "127.0.0.1"}) // Example for local networks
	r.LoadHTMLGlob("templates/*")

	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl", nil)
	})

	// Upload and parse OpenAPI spec
	r.POST("/upload", func(c *gin.Context) {
		file, err := c.FormFile("oas")
		if err != nil {
			c.String(http.StatusBadRequest, "Error retrieving file")
			return
		}

		f, err := file.Open()
		if err != nil {
			c.String(http.StatusInternalServerError, "Could not open file")
			return
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			c.String(http.StatusInternalServerError, "Error reading file")
			return
		}

		loader := openapi3.NewLoader()
		spec, err := loader.LoadFromData(data)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid OAS file")
			return
		}

		loadedSpec = spec // Store the loaded OpenAPI spec in memory

		endpoints := []string{}
		if spec.Paths != nil {
			for path, pathItem := range spec.Paths.Map() {
				for method := range pathItem.Operations() {
					endpoints = append(endpoints, fmt.Sprintf("%s %s", method, path))
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"endpoints": endpoints})
	})

	// Create new OpenAPI spec based on selected endpoints
	r.POST("/generate", func(c *gin.Context) {
		var selectedEndpoints []struct {
			Endpoint    string `json:"endpoint"`
			Integration string `json:"integration"`
		}
		if err := c.BindJSON(&selectedEndpoints); err != nil {
			c.String(http.StatusBadRequest, "Invalid request")
			return
		}

		if loadedSpec == nil {
			c.String(http.StatusInternalServerError, "No OpenAPI spec loaded")
			return
		}

		newSpec := &openapi3.T{
			OpenAPI: "3.0.0",
			Info:    loadedSpec.Info,
			Paths:   openapi3.NewPaths(),
		}

		for _, ep := range selectedEndpoints {
			for path, pathItem := range loadedSpec.Paths.Map() {
				for method, operation := range pathItem.Operations() {
					if fmt.Sprintf("%s %s", method, path) == ep.Endpoint {
						if _, exists := newSpec.Paths.Map()[path]; !exists {
							newSpec.Paths.Set(path, &openapi3.PathItem{})
						}
						operation.Extensions = map[string]interface{}{
							"x-amazon-apigateway-integration": map[string]string{
								"type":       ep.Integration,
								"uri":        "https://example.com", // Replace with actual URI
								"httpMethod": method,
							},
						}
						newSpec.Paths.Map()[path].SetOperation(method, operation)
					}
				}
			}
		}

		jsonSpec, err := json.MarshalIndent(newSpec, "", "  ")
		if err != nil {
			c.String(http.StatusInternalServerError, "Error generating OpenAPI spec")
			return
		}

		c.Data(http.StatusOK, "application/json", jsonSpec)
	})

	r.Run(":8080")
}
