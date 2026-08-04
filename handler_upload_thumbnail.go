package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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

	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse thumbnail", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't form thumbnail file", err)
		return
	}
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}

	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Thumbnail is in unsupported format: %s", mediaType), err)
		return
	}
	mediaInfo := strings.Split(mediaType, "/")
	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't access video data", err)
		return
	}

	randBytes := make([]byte, 32)
	rand.Read(randBytes)
	fileName := base64.RawURLEncoding.EncodeToString(randBytes)
	videoFileURL := fmt.Sprintf("%s.%s", fileName, mediaInfo[1])
	fullFileURL := filepath.Join(cfg.assetsRoot, videoFileURL)
	newFile, err := os.Create(fullFileURL)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't create thumbnail file", err)
		return
	}
	io.Copy(newFile, file)

	dataURL := fmt.Sprintf("http://localhost:%s/%s", cfg.port, fullFileURL)

	videoData.ThumbnailURL = &dataURL
	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't update video data", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
