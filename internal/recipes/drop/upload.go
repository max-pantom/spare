package drop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const multipartAllowance = int64(1024 * 1024)

func (s *server) upload(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.Header().Set("Allow", "POST")
		writeDropError(response, http.StatusMethodNotAllowed, "Use the upload form to send a file.")
		return
	}
	select {
	case s.uploadSlot <- struct{}{}:
		defer func() { <-s.uploadSlot }()
	default:
		writeDropError(response, http.StatusConflict, "Another file is being received. Wait for it to finish, then try again.")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, s.maximumFileSize+multipartAllowance)
	reader, err := request.MultipartReader()
	if err != nil {
		writeDropError(response, http.StatusBadRequest, "Choose one file to upload.")
		return
	}
	var stored fileEntry
	found := false
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			writeDropError(response, http.StatusBadRequest, "Unable to read this upload. Choose the file again.")
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		if found {
			_ = part.Close()
			writeDropError(response, http.StatusBadRequest, "Upload one file at a time.")
			return
		}
		found = true
		entry, storeErr := s.storeUpload(part.FileName(), part)
		_ = part.Close()
		if storeErr != nil {
			if errors.Is(storeErr, errInvalidFilename) {
				writeDropError(response, http.StatusBadRequest, "Choose a file with a safe, visible name.")
				return
			}
			var tooLarge *fileTooLargeError
			if errors.As(storeErr, &tooLarge) {
				writeDropError(response, http.StatusRequestEntityTooLarge, fmt.Sprintf("Choose a file smaller than %s.", formatBytes(s.maximumFileSize)))
				return
			}
			var noSpace *notEnoughStorageError
			if errors.As(storeErr, &noSpace) {
				writeDropError(response, http.StatusInsufficientStorage, "This computer does not have enough available storage for that file.")
				return
			}
			writeDropError(response, http.StatusInternalServerError, "Unable to save this file. Check that the destination folder is writable.")
			return
		}
		stored = entry
	}
	if !found {
		writeDropError(response, http.StatusBadRequest, "Choose one file to upload.")
		return
	}
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(response).Encode(stored)
}

func (s *server) storeUpload(name string, source io.Reader) (fileEntry, error) {
	availableBefore := availableStorage(s.root)
	target, err := collisionFreePath(s.root, name)
	if err != nil {
		return fileEntry{}, err
	}
	temporary, err := os.CreateTemp(s.root, ".spare-upload-*")
	if err != nil {
		return fileEntry{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	written, copyErr := io.Copy(temporary, io.LimitReader(source, s.maximumFileSize+1))
	closeErr := temporary.Close()
	if copyErr != nil {
		return fileEntry{}, copyErr
	}
	if closeErr != nil {
		return fileEntry{}, closeErr
	}
	if written > s.maximumFileSize {
		return fileEntry{}, &fileTooLargeError{}
	}
	if availableBefore > 0 && uint64(written) > availableBefore {
		return fileEntry{}, &notEnoughStorageError{}
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return fileEntry{}, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return fileEntry{}, err
	}
	return fileEntry{
		Name:       filepath.Base(target),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC(),
		URL:        "/files/" + pathEscape(filepath.Base(target)),
	}, nil
}

type fileTooLargeError struct{}

func (e *fileTooLargeError) Error() string {
	return "file is too large"
}

type notEnoughStorageError struct{}

func (e *notEnoughStorageError) Error() string {
	return "not enough storage"
}

func writeDropError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": message})
}
