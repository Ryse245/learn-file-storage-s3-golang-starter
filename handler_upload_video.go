package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 30
	var reader io.ReadCloser
	http.MaxBytesReader(w, reader, maxMemory)
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

	fmt.Println("uploading video", videoID, "by user", userID)

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't access video data", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't form video file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Thumbnail is in unsupported format: %s", mediaType), err)
		return
	}
	vidFile, err := os.CreateTemp("", "tubely_upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't create new video file", err)
		return
	}
	defer os.Remove(vidFile.Name())
	defer vidFile.Close()

	io.Copy(vidFile, file)
	vidFile.Seek(0, io.SeekStart)
	aspectRatio, err := GetVideoAspectRatio(vidFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse aspect ratio", err)
	}
	var aspectStr string = "other"
	switch aspectRatio {
	case "16:9":
		aspectStr = "landscape"
	case "9:16":
		aspectStr = "portrait"
	}

	fastVid, err := ProcessVideoForFastStart(vidFile.Name())
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to process for fast start", err)
	}
	fastVidFile, err := os.Open(fastVid)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to open fast start file", err)
	}
	defer os.Remove(fastVidFile.Name())
	defer fastVidFile.Close()

	randBytes := make([]byte, 32)
	rand.Read(randBytes)
	keyIGuess := base64.RawURLEncoding.EncodeToString(randBytes)
	fullKey := aspectStr + "/" + keyIGuess
	_, err = cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{Bucket: &cfg.s3Bucket, Key: &fullKey, Body: fastVidFile, ContentType: &mediaType})
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't add object to bucket", err)
		return
	}

	awsVidUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fullKey)
	videoData.VideoURL = &awsVidUrl
	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't update video data", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)

}
