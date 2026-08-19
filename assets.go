package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

type VideoMetaData struct {
	Streams []struct {
		Index              int    `json:"index"`
		CodecName          string `json:"codec_name,omitempty"`
		CodecLongName      string `json:"codec_long_name,omitempty"`
		Profile            string `json:"profile,omitempty"`
		CodecType          string `json:"codec_type"`
		CodecTagString     string `json:"codec_tag_string"`
		CodecTag           string `json:"codec_tag"`
		Width              int    `json:"width,omitempty"`
		Height             int    `json:"height,omitempty"`
		CodedWidth         int    `json:"coded_width,omitempty"`
		CodedHeight        int    `json:"coded_height,omitempty"`
		ClosedCaptions     int    `json:"closed_captions,omitempty"`
		FilmGrain          int    `json:"film_grain,omitempty"`
		HasBFrames         int    `json:"has_b_frames,omitempty"`
		SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`
		DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
		PixFmt             string `json:"pix_fmt,omitempty"`
		Level              int    `json:"level,omitempty"`
		ColorRange         string `json:"color_range,omitempty"`
		ColorSpace         string `json:"color_space,omitempty"`
		ColorTransfer      string `json:"color_transfer,omitempty"`
		ColorPrimaries     string `json:"color_primaries,omitempty"`
		ChromaLocation     string `json:"chroma_location,omitempty"`
		FieldOrder         string `json:"field_order,omitempty"`
		Refs               int    `json:"refs,omitempty"`
		IsAvc              string `json:"is_avc,omitempty"`
		NalLengthSize      string `json:"nal_length_size,omitempty"`
		ID                 string `json:"id"`
		RFrameRate         string `json:"r_frame_rate"`
		AvgFrameRate       string `json:"avg_frame_rate"`
		TimeBase           string `json:"time_base"`
		StartPts           int    `json:"start_pts"`
		StartTime          string `json:"start_time"`
		DurationTs         int    `json:"duration_ts"`
		Duration           string `json:"duration"`
		BitRate            string `json:"bit_rate,omitempty"`
		BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
		NbFrames           string `json:"nb_frames"`
		ExtradataSize      int    `json:"extradata_size"`
		SampleFmt          string `json:"sample_fmt,omitempty"`
		SampleRate         string `json:"sample_rate,omitempty"`
		Channels           int    `json:"channels,omitempty"`
		ChannelLayout      string `json:"channel_layout,omitempty"`
		BitsPerSample      int    `json:"bits_per_sample,omitempty"`
		InitialPadding     int    `json:"initial_padding,omitempty"`
	} `json:"streams"`
}

func GetVideoAspectRatio(filepath string) (string, error) {
	var buffer bytes.Buffer
	cmdOut := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	cmdOut.Stdout = &buffer
	cmdOut.Run()
	var vidMetaData VideoMetaData
	if err := json.Unmarshal(buffer.Bytes(), &vidMetaData); err != nil {
		return "error", err
	}
	var ratio float64 = float64(vidMetaData.Streams[0].Width) / float64(vidMetaData.Streams[0].Height)
	if math.Abs(ratio-(16.0/9.0)) < 0.001 {
		return "16:9", nil
	} else if math.Abs(ratio-(9.0/16.0)) < 0.001 {
		return "9:16", nil
	}
	return "other", nil
}

func ProcessVideoForFastStart(filepath string) (string, error) {
	outputStr := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputStr)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outputStr, nil
}

func GeneratePresignedUrl(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignCli := s3.NewPresignClient(s3Client)
	retShit, err := presignCli.PresignGetObject(context.Background(), &s3.GetObjectInput{Bucket: &bucket, Key: &key}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return retShit.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	b4, key, _ := strings.Cut(*video.VideoURL, ",")
	_, bucket, _ := strings.Cut(b4, ".com/")

	preUrl, err := GeneratePresignedUrl(cfg.s3Client, bucket, key, time.Hour)
	if err != nil {
		return database.Video{}, err
	}
	video.VideoURL = &preUrl
	return video, nil
}
