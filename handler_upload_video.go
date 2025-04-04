package main

import (
	"net/http"
	"fmt"
	"io"
	"mime"
	"os"
	"context"
	//"encoding/base64"
	//"config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	//setting limite massimo grandezza video
	const maxMemory = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body ,maxMemory)
	//parso l'uuid del video
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}
	//validazione standard
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
	//mi prendo i metadati del video da DB
	videoMetadata, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to query the video", err)
		return
	}
	//controllo che lo user sia l'owner del video. KO altrimenti
	if videoMetadata.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User not owner of video", err)
		return
	}
	//mi parso il file per recuperare tutto. Chiudo alla fine
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	//prendo il mediaType
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}

	//controllo sul mediaType del video (deve essere mp4)
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}
	//creo temporaneamente il file in locale
	tmpVideo, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file on local", err)
		return
	}
	//rimuove il file alla fine
	defer os.Remove(tmpVideo.Name())
	//chiude il file. la logica è LIFO, quindi prima lo chiude e poi lo rimuove
	defer tmpVideo.Close()

	//copia il contenuto di 'file' nel mio file temporaneo
	if _, err = io.Copy(tmpVideo, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(tmpVideo.Name())
	if _, err = io.Copy(tmpVideo, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error with aspect ratio", err)
		return
	}
	var prefixRatio string
	if aspectRatio == "16:9"{
		prefixRatio = "landscape/"
	} else if aspectRatio == "9:16"{
		prefixRatio = "portrait/"
	} else {
		prefixRatio = "other/"
	}
	//setto il file pointer all'inizio
	tmpVideo.Seek(0, io.SeekStart)

	//setto il nome/path del file

	processedVideoPath, _ := processVideoForFastStart(tmpVideo.Name())
	defer os.Remove(processedVideoPath)

	processedVideoFile, err := os.Open(processedVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error opening processed video", err)
		return
	}
	defer processedVideoFile.Close()

	var uploadFileName string
	uploadFileName = prefixRatio + getAssetPath(mediaType)
	//creazione parametri per l'insert
	inputParams := s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key:	&uploadFileName,
		Body:	processedVideoFile,
		ContentType: &mediaType,	   
	}

	//metto il file su s3
	cfg.s3Client.PutObject(context.TODO(), &inputParams)

	dataUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, uploadFileName)
	videoMetadata.VideoURL = &dataUrl
	err = cfg.db.UpdateVideo(videoMetadata)

	respondWithJSON(w, http.StatusOK, videoMetadata)
}
