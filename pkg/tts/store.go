package tts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// AudioStore writes synthesized audio files to a base directory (e.g. a Longhorn PVC mount).
type AudioStore struct {
	BasePath string // e.g. "/audio"
}

// NewAudioStore creates an AudioStore, ensuring the directory exists.
func NewAudioStore(basePath string) (*AudioStore, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", basePath, err)
	}
	return &AudioStore{BasePath: basePath}, nil
}

// Save writes audio data to {BasePath}/{jobID}.mp3 and returns the file path
// and a relative URL suitable for serving.
func (as *AudioStore) Save(jobID uuid.UUID, data []byte) (filePath, audioURL string, err error) {
	filename := jobID.String() + ".mp3"
	filePath = filepath.Join(as.BasePath, filename)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", "", fmt.Errorf("write %s: %w", filePath, err)
	}
	audioURL = "/audio/" + filename
	return filePath, audioURL, nil
}
