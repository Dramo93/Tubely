package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"os/exec"
	"log"
	"encoding/json"
	"bytes"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {

	type ffprobeOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams",filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	var streamsToAnalize ffprobeOutput
	if err := json.Unmarshal(out.Bytes(), &streamsToAnalize); err != nil {
        log.Fatalf("Error unmarshalling JSON: %v", err)
    }

	tolerance := 0.01
	ratio := float64(streamsToAnalize.Streams[0].Width) / float64(streamsToAnalize.Streams[0].Height)

	if (ratio > 16.0/9.0-tolerance) && (ratio < 16.0/9.0+tolerance) {
		return "16:9", nil
	} else if (ratio > 9.0/16.0-tolerance) && (ratio < 9.0/16.0+tolerance) {
		return "9:16", nil
	} else {
		return "other", nil
	}

}

func processVideoForFastStart(filePath string) (string, error){
	outputPath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath )
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	return outputPath, nil
}
