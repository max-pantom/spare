package jobpackage

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spare-run/spare/internal/artifacts"
)

func TestSignVerifyAndRejectTampering(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "job.sp")
	if err := artifacts.PackDirectory(source, packagePath); err != nil {
		t.Fatal(err)
	}
	envelope, err := Sign(packagePath, privateKey, "0.1.1-alpha.3")
	if err != nil {
		t.Fatal(err)
	}
	verifier := NewVerifier(map[string]ed25519.PublicKey{
		DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
	})
	verified, err := verifier.Verify(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Digest != envelope.Digest || verified.MinimumSpare != "0.1.1-alpha.3" {
		t.Fatalf("verification = %#v", verified)
	}

	rewriteEntry(t, packagePath, "spare.yml", []byte("changed"))
	if _, err := verifier.Verify(packagePath); err == nil {
		t.Fatal("expected changed content to fail verification")
	}
}

func TestVerifyRejectsChangedSignatureMetadata(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "job.sp")
	if err := artifacts.PackDirectory(source, packagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := Sign(packagePath, privateKey, "0.1.1-alpha.3"); err != nil {
		t.Fatal(err)
	}
	data, err := artifacts.ReadFile(packagePath, SignatureFile)
	if err != nil {
		t.Fatal(err)
	}
	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope.MinimumSpare = "0.0.1"
	data, err = json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	rewriteEntry(t, packagePath, SignatureFile, data)

	verifier := NewVerifier(map[string]ed25519.PublicKey{
		DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
	})
	if _, err := verifier.Verify(packagePath); err == nil {
		t.Fatal("expected changed signature metadata to fail verification")
	}
}

func TestVerifyRejectsTrailingSignatureData(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "spare.yml"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join(t.TempDir(), "job.sp")
	if err := artifacts.PackDirectory(source, packagePath); err != nil {
		t.Fatal(err)
	}
	if _, err := Sign(packagePath, privateKey, "0.1.1-alpha.3"); err != nil {
		t.Fatal(err)
	}
	data, err := artifacts.ReadFile(packagePath, SignatureFile)
	if err != nil {
		t.Fatal(err)
	}
	rewriteEntry(t, packagePath, SignatureFile, append(data, []byte(`{}`)...))

	verifier := NewVerifier(map[string]ed25519.PublicKey{
		DefaultKeyID: privateKey.Public().(ed25519.PublicKey),
	})
	if _, err := verifier.Verify(packagePath); err == nil {
		t.Fatal("expected trailing signature data to fail verification")
	}
}

func TestLoadPrivateKeyRejectsRawSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed")
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(make([]byte, ed25519.SeedSize))), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(path); err == nil {
		t.Fatal("expected non-PEM key to be rejected")
	}
}

func rewriteEntry(t *testing.T, packagePath, name string, replacement []byte) {
	t.Helper()
	source, err := zip.OpenReader(packagePath)
	if err != nil {
		t.Fatal(err)
	}
	temporary := packagePath + ".tmp"
	output, err := os.Create(temporary)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(output)
	for _, file := range source.File {
		header := file.FileHeader
		entry, err := writer.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name == name {
			if _, err := entry.Write(replacement); err != nil {
				t.Fatal(err)
			}
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(entry, reader); err != nil {
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(temporary, packagePath); err != nil {
		t.Fatal(err)
	}
}
