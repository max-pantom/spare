package jobpackage

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spare-run/spare/internal/artifacts"
)

const (
	SignatureFile    = "spare-signature.json"
	SignatureSchema  = "spare.package-signature/v1"
	DefaultKeyID     = "spare-preview-2026"
	DefaultPublisher = "Spare"
)

// DefaultPublicKeyBase64 is the public half of Spare's preview catalog key.
// The private half is kept outside the repository and supplied to release
// builds through SPARE_CATALOG_SIGNING_KEY.
const DefaultPublicKeyBase64 = "0p8BLJiGhqkte2mkcqosmc3UCrWk3H3mZj41A8f66cY="

type Envelope struct {
	Schema       string `json:"schema"`
	Publisher    string `json:"publisher"`
	KeyID        string `json:"keyId"`
	MinimumSpare string `json:"minimumSpareVersion"`
	Digest       string `json:"digest"`
	Signature    string `json:"signature"`
	SignedAt     string `json:"signedAt"`
}

type Verification struct {
	Publisher    string
	KeyID        string
	MinimumSpare string
	Digest       string
	Signature    string
}

type Verifier struct {
	keys map[string]ed25519.PublicKey
}

func DefaultVerifier() (*Verifier, error) {
	raw, err := base64.StdEncoding.DecodeString(DefaultPublicKeyBase64)
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, errors.New("the embedded catalog public key is invalid")
	}
	return NewVerifier(map[string]ed25519.PublicKey{
		DefaultKeyID: ed25519.PublicKey(raw),
	}), nil
}

func NewVerifier(keys map[string]ed25519.PublicKey) *Verifier {
	copied := make(map[string]ed25519.PublicKey, len(keys))
	for id, key := range keys {
		copied[id] = append(ed25519.PublicKey(nil), key...)
	}
	return &Verifier{keys: copied}
}

func (v *Verifier) Verify(packagePath string) (Verification, error) {
	if v == nil {
		return Verification{}, errors.New("package verifier is unavailable")
	}
	data, err := artifacts.ReadFileLimit(packagePath, SignatureFile, 64*1024)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Verification{}, errors.New("this job package is unsigned")
		}
		return Verification{}, err
	}
	var envelope Envelope
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return Verification{}, fmt.Errorf("read package signature: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Verification{}, errors.New("package signature contains trailing data")
	}
	if envelope.Schema != SignatureSchema {
		return Verification{}, fmt.Errorf("unsupported package signature schema %q", envelope.Schema)
	}
	if envelope.Publisher != DefaultPublisher {
		return Verification{}, fmt.Errorf("untrusted package publisher %q", envelope.Publisher)
	}
	key, ok := v.keys[envelope.KeyID]
	if !ok {
		return Verification{}, fmt.Errorf("untrusted package signing key %q", envelope.KeyID)
	}
	digest, err := artifacts.PackageDigest(packagePath, SignatureFile)
	if err != nil {
		return Verification{}, err
	}
	if !strings.EqualFold(digest, envelope.Digest) {
		return Verification{}, errors.New("the package contents do not match its signature")
	}
	signature, err := base64.StdEncoding.DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return Verification{}, errors.New("the package signature is invalid")
	}
	if !ed25519.Verify(key, signatureMessage(envelope), signature) {
		return Verification{}, errors.New("the package signature could not be verified")
	}
	return Verification{
		Publisher:    envelope.Publisher,
		KeyID:        envelope.KeyID,
		MinimumSpare: envelope.MinimumSpare,
		Digest:       digest,
		Signature:    envelope.Signature,
	}, nil
}

func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("catalog signing key is not PEM encoded")
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := value.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("catalog signing key is not Ed25519")
	}
	return key, nil
}

func Sign(packagePath string, key ed25519.PrivateKey, minimumSpare string) (Envelope, error) {
	if len(key) != ed25519.PrivateKeySize {
		return Envelope{}, errors.New("catalog signing key is invalid")
	}
	if strings.TrimSpace(minimumSpare) == "" {
		return Envelope{}, errors.New("minimum Spare version is required")
	}
	files, err := artifacts.ListFiles(packagePath)
	if err != nil {
		return Envelope{}, err
	}
	for _, file := range files {
		if file.Name == SignatureFile {
			return Envelope{}, errors.New("package is already signed")
		}
	}
	digest, err := artifacts.PackageDigest(packagePath, SignatureFile)
	if err != nil {
		return Envelope{}, err
	}
	envelope := Envelope{
		Schema:       SignatureSchema,
		Publisher:    DefaultPublisher,
		KeyID:        DefaultKeyID,
		MinimumSpare: minimumSpare,
		Digest:       digest,
		SignedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	envelope.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, signatureMessage(envelope)))
	encoded, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return Envelope{}, err
	}
	encoded = append(encoded, '\n')
	if err := appendSignature(packagePath, encoded); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

func signatureMessage(envelope Envelope) []byte {
	return []byte(strings.Join([]string{
		envelope.Schema,
		envelope.Publisher,
		envelope.KeyID,
		envelope.MinimumSpare,
		strings.ToLower(envelope.Digest),
		envelope.SignedAt,
	}, "\n"))
}

func appendSignature(packagePath string, signature []byte) error {
	source, err := zip.OpenReader(packagePath)
	if err != nil {
		return err
	}
	defer source.Close()
	if err := os.MkdirAll(filepath.Dir(packagePath), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(packagePath), ".spare-signed-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	writer := zip.NewWriter(temporary)

	files := append([]*zip.File(nil), source.File...)
	sort.Slice(files, func(left, right int) bool {
		return files[left].Name < files[right].Name
	})
	for _, file := range files {
		header := file.FileHeader
		header.SetModTime(time.Unix(0, 0).UTC())
		output, err := writer.CreateHeader(&header)
		if err != nil {
			_ = writer.Close()
			_ = temporary.Close()
			return err
		}
		input, err := file.Open()
		if err != nil {
			_ = writer.Close()
			_ = temporary.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeErr := input.Close()
		if copyErr != nil {
			_ = writer.Close()
			_ = temporary.Close()
			return copyErr
		}
		if closeErr != nil {
			_ = writer.Close()
			_ = temporary.Close()
			return closeErr
		}
	}
	header := &zip.FileHeader{Name: SignatureFile, Method: zip.Deflate}
	header.SetMode(0o600)
	header.SetModTime(time.Unix(0, 0).UTC())
	output, err := writer.CreateHeader(header)
	if err != nil {
		_ = writer.Close()
		_ = temporary.Close()
		return err
	}
	if _, err := output.Write(signature); err != nil {
		_ = writer.Close()
		_ = temporary.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return err
	}
	if err := source.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, packagePath); err == nil {
		return nil
	}
	if err := os.Remove(packagePath); err != nil {
		return err
	}
	return os.Rename(temporaryPath, packagePath)
}
