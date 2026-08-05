package artifacts

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
)

// PackageDigest returns a stable digest of the logical package contents. ZIP
// container metadata and the signature envelope itself are intentionally not
// included, so a package can be signed after it has been assembled.
func PackageDigest(packagePath, excludedName string) (string, error) {
	if _, err := ListFiles(packagePath); err != nil {
		return "", err
	}
	archive, err := zip.OpenReader(packagePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()

	files := append([]*zip.File(nil), archive.File...)
	sort.Slice(files, func(left, right int) bool {
		return files[left].Name < files[right].Name
	})
	hash := sha256.New()
	for _, file := range files {
		name, err := cleanArchivePath(file.Name)
		if err != nil {
			return "", err
		}
		if file.FileInfo().IsDir() || name == excludedName {
			continue
		}
		if err := binary.Write(hash, binary.BigEndian, uint64(len(name))); err != nil {
			return "", err
		}
		if _, err := io.WriteString(hash, name); err != nil {
			return "", err
		}
		if err := binary.Write(hash, binary.BigEndian, file.UncompressedSize64); err != nil {
			return "", err
		}
		reader, err := file.Open()
		if err != nil {
			return "", err
		}
		written, copyErr := io.Copy(hash, io.LimitReader(reader, maxPackageFileSize+1))
		closeErr := reader.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		if written != int64(file.UncompressedSize64) {
			return "", fmt.Errorf("package file size changed while hashing: %s", name)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
