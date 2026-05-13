package main

import (
	"errors"
	"io"

	"github.com/yeka/zip"
)

// CheckPassword attempts to open the first file in the ZIP using the provided password.
func CheckPassword(zipPath, password string) (bool, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return false, err
	}
	defer reader.Close()

	if len(reader.File) == 0 {
		return false, errors.New("Zip File is Empty")
	}

	file := reader.File[0]
	if !file.IsEncrypted() {
		return false, errors.New("The first file in the zip is not Encrypted")
	}

	file.SetPassword(password)

	rc, err := file.Open()
	if err != nil {
		return false, nil // Incorrect password usually results in an error here
	}
	defer rc.Close()

	buf := make([]byte, 1)
	_, err = rc.Read(buf)

	if err != nil && err != io.EOF {
		return false, nil
	}

	return true, nil
}
