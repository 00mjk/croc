package main

import (
	"os"
	"testing"
)

func TestSplitFile(t *testing.T) {
	err := SplitFile("testing_data/README.md", 3)
	if err != nil {
		t.Error(err)
	}
	os.Remove("testing_data/README.md.0")
	os.Remove("testing_data/README.md.1")
}

func TestFileSize(t *testing.T) {
	t.Run("File is ok ", func(t *testing.T) {
		_, err := FileSize("testing_data/README.md")
		if err != nil {
			t.Errorf("should pass with no error, got: %v", err)
		}
	})
	t.Run("File does not exist", func(t *testing.T) {
		s, err := FileSize("testing_data/someStrangeFile")
		if err == nil {
			t.Error("should return an error")
		}
		if s > 0 {
			t.Errorf("size should be 0, got: %d", s)
		}
	})
}

func TestHashFile(t *testing.T) {
	t.Run("Hash created successfully", func(t *testing.T) {
		h, err := HashFile("testing_data/README.md")
		if err != nil {
			t.Errorf("should pass with no error, got: %v", err)
		}
		if len(h) != 32 {
			t.Errorf("invalid md5 hash, length should be 32 got: %d", len(h))
		}
	})
	t.Run("File does not exist", func(t *testing.T) {
		h, err := HashFile("testing_data/someStrangeFile")
		if err == nil {
			t.Error("should return an error")
		}
		if len(h) > 0 {
			t.Errorf("hash length should be 0, got: %d", len(h))
		}
		if h != "" {
			t.Errorf("hash should be empty string, got: %s", h)
		}
	})
}

func TestCopyFileContents(t *testing.T) {
	t.Run("Content copied successfully", func(t *testing.T) {
		f1 := "testing_data/README.md"
		f2 := "testing_data/CopyOfREADME.md"
		err := copyFileContents(f1, f2)
		if err != nil {
			t.Errorf("should pass with no error, got: %v", err)
		}
		f1Length, err := FileSize(f1)
		if err != nil {
			t.Errorf("can't get file nr1 size: %v", err)
		}
		f2Length, err := FileSize(f2)
		if err != nil {
			t.Errorf("can't get file nr2 size: %v", err)
		}

		if f1Length != f2Length {
			t.Errorf("size of both files should be same got: file1: %d file2: %d", f1Length, f2Length)
		}
		os.Remove(f2)
	})
}

func TestCopyFile(t *testing.T) {
	t.Run("Content copied successfully", func(t *testing.T) {
		f1 := "testing_data/README.md"
		f2 := "testing_data/CopyOfREADME.md"
		err := CopyFile(f1, f2)
		if err != nil {
			t.Errorf("should pass with no error, got: %v", err)
		}
		f1Length, err := FileSize(f1)
		if err != nil {
			t.Errorf("can't get file nr1 size: %v", err)
		}
		f2Length, err := FileSize(f2)
		if err != nil {
			t.Errorf("can't get file nr2 size: %v", err)
		}

		if f1Length != f2Length {
			t.Errorf("size of both files should be same got: file1: %d file2: %d", f1Length, f2Length)
		}
		os.Remove(f2)
	})
}
