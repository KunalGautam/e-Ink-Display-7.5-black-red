package widget

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"strings"

	"golang.org/x/image/draw"
)

type ImageWidget struct{}

type ImageConfig struct {
	URL    string `json:"url"`
	Base64 string `json:"base64"`
}

func (w *ImageWidget) Render(ctx context.Context, rCtx *RenderContext) error {
	var src image.Image
	var err error

	inputURL := ""
	inputBase64 := ""

	// Read static parameters
	if rCtx.CustomConfig != "" {
		var cfg ImageConfig
		if err := json.Unmarshal([]byte(rCtx.CustomConfig), &cfg); err == nil {
			inputURL = cfg.URL
			inputBase64 = cfg.Base64
		}
	}

	// Override with dynamic MQTT payload if present
	if rCtx.LatestData != "" {
		if strings.HasPrefix(rCtx.LatestData, "data:image/") || len(rCtx.LatestData) > 500 {
			inputBase64 = rCtx.LatestData
		} else {
			inputURL = rCtx.LatestData
		}
	}

	if inputBase64 != "" {
		src, err = decodeBase64Image(inputBase64)
	} else if inputURL != "" {
		src, err = downloadImage(inputURL)
	}

	if err != nil || src == nil {
		rCtx.Ctx.SetHexColor(rCtx.ColorFG)
		rCtx.Ctx.DrawString("Image error.", 5, 20)
		return nil
	}

	// Resize to fit widget boundaries using standard scale
	rect := image.Rect(0, 0, rCtx.Width, rCtx.Height)
	dst := image.NewRGBA(rect)
	draw.CatmullRom.Scale(dst, rect, src, src.Bounds(), draw.Over, nil)

	rCtx.Ctx.DrawImage(dst, 0, 0)
	return nil
}

func decodeBase64Image(data string) (image.Image, error) {
	if idx := strings.Index(data, ";base64,"); idx != -1 {
		data = data[idx+8:]
	}
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(data))
	img, _, err := image.Decode(reader)
	return img, err
}

func downloadImage(urlStr string) (image.Image, error) {
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	return img, err
}
