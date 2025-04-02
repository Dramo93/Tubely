package main

import (
	"fmt"
	"net/http"
	"io"
	"path/filepath"
	"os"
	"crypto/rand"
	"encoding/base64"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	//const to 10 MB. bit-shifted to the left 20 times to get an int the stores the proper number of bytes
	//it's like doing 10 * 1024 * 1024
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	if mediaType == "" {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}
		// Extract the file extension from the Content-Type
	var fileExt string
	switch mediaType {
	case "image/jpeg", "image/jpg":
		fileExt = ".jpg"
	case "image/png":
		fileExt = ".png"
	// Add other cases as needed
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported file type", nil)
		return
	}
	filePathIncomplete, _ := MakeRandomizedFilePath()
	filePathForThumb := filepath.Join(cfg.assetsRoot, filePathIncomplete + fileExt)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}


	fileThumb, err := os.Create(filePathForThumb)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	defer fileThumb.Close()

	if _, err := io.Copy(fileThumb, file); err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to create file", err)
		return
	}



	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to query the video", err)
		return
	}

	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not owner of video", err)
		return
	}

	dataUrl := fmt.Sprintf("http://localhost:%s/assets/%s%s", cfg.port, filePathForThumb, fileExt)
	videoMetadata.ThumbnailURL = &dataUrl
	err = cfg.db.UpdateVideo(videoMetadata)

	respondWithJSON(w, http.StatusOK, videoMetadata)
}

func MakeRandomizedFilePath() (string, error) {
	token := make([]byte, 32)
	_, err := rand.Read(token)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token), nil
}
