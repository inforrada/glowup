package main

import (
	"context"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/url" // Import necesario para parsear la URL
	"os"
	"os/signal"
	"path/filepath" // Import necesario para manipular rutas
	"strings"
	"syscall"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

func main() {
	outputDir := "output"
	urlFlag := flag.String("url", "", "url of the image to restore")
	contentType := flag.String("contentType", "image/jpeg", "content type of the image (default: image/jpeg)")
	flag.Parse()

	var urls []string
	if *urlFlag != "" {
		urls = append(urls, *urlFlag)
	} else {
		// Intentar leer el fichero si no se pasó la flag --url
		fileData, err := os.ReadFile("imagenes.txt")
		if err != nil {
			log.Fatalf("No se proporcionó --url y no se pudo leer imagenes.txt: %v", err)
		}
		// Dividir por líneas y filtrar líneas vacías
		lines := strings.Split(string(fileData), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				urls = append(urls, trimmed)
			}
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Initialize Genkit with the Vertex AI plugin
	g := genkit.Init(ctx,
		genkit.WithPlugins(&googlegenai.VertexAI{}),
	)
	type Input struct {
		URL         string `json:"url"`
		ContentType string `json:"contentType"`
	}
	glowup := genkit.DefineFlow(g, "glowUp", func(ctx context.Context, input Input) (string, error) {
		prompt := genkit.LookupPrompt(g, "glowup")
		if prompt == nil {
			return "", errors.New("prompt 'glowup' not found")
		}
		resp, err := prompt.Execute(ctx, ai.WithInput(input))
		if err != nil {
			return "", fmt.Errorf("generation failed: %w", err)
		}

		return resp.Media(), nil
	})
	for _, uStr := range urls { // Cambiado a 'uStr' para evitar conflictos
		uStr = strings.TrimSpace(uStr)
		if uStr == "" {
			continue
		}

		fmt.Printf("Procesando: %s\n", uStr)

		// IMPORTANTE: usa 'uStr' aquí, no *urlFlag
		out, err := glowup.Run(ctx, Input{URL: uStr, ContentType: *contentType})
		if err != nil {
			log.Printf("Error en flujo: %v", err)
			continue
		}

		data, ext, err := decode(out)
		if err != nil {
			log.Printf("Error decodificando: %v", err)
			continue
		}

		// 4. Parsea la URL usando 'uStr'
		parsedURL, err := url.Parse(uStr)
		if err != nil {
			log.Printf("Error parseando URL: %v", err)
			continue
		}

		// ... lógica de guardado ...
		baseName := filepath.Base(parsedURL.Path)
		nameWithoutExt := strings.TrimSuffix(baseName, filepath.Ext(baseName))
		filename := filepath.Join(outputDir, fmt.Sprintf("%s-4k%s", nameWithoutExt, ext))

		os.WriteFile(filename, data, 0644)
	}
}

// decode returns the decoded data and the file extension appropriate for the mime type
func decode(text string) ([]byte, string, error) {
	if !strings.HasPrefix(text, "data:") {
		return nil, "", errors.New("unsupported enconding format")
	}
	text = strings.TrimPrefix(text, "data:")
	parts := strings.Split(text, ";base64,")

	mimeType := parts[0]
	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", err
	}

	ext, err := mime.ExtensionsByType(mimeType)
	if err != nil {
		return nil, "", err
	}

	return decoded, ext[0], nil
}
