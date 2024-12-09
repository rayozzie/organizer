package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultInputDir  = "Pictures"
	defaultOutputDir = "Desktop/organizer"
)

type FileInfo struct {
	Name         string                   `json:"name"`
	Extension    string                   `json:"extension"`
	Size         int64                    `json:"size"`
	InputPath    string                   `json:"input_path"`
	ModTime      time.Time                `json:"modified"`
	CapturedTime time.Time                `json:"captured"`
	ExifData     map[string]ExifDataEntry `json:"exif,omitempty"`
}

func main() {
	// Set default input and output directories
	inputDir := filepath.Join(os.Getenv("HOME"), defaultInputDir)
	outputDir := filepath.Join(os.Getenv("HOME"), defaultOutputDir)
	outputDirRecognized := filepath.Join(outputDir, "output")
	outputDirError := filepath.Join(outputDir, "error")

	// Override input/output directories with command-line arguments, if provided
	if len(os.Args) == 3 {
		inputDir = os.Args[1]
		outputDir = os.Args[2]
		outputDirRecognized = filepath.Join(outputDir, "output")
		outputDirError = filepath.Join(outputDir, "error")
	}

	// Create the output and error directories
	for _, dir := range []string{outputDirRecognized, outputDirError} {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			log.Fatalf("Error creating directory '%s': %v", dir, err)
		}
	}

	// Enumerate files in the input directory
	fileInfos := []FileInfo{}
	err := filepath.Walk(inputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Extract file metadata
		fileExt := strings.ToLower(strings.TrimPrefix(filepath.Ext(info.Name()), "."))
		fileInfo := FileInfo{
			Name:         info.Name(),
			Extension:    fileExt,
			Size:         info.Size(),
			InputPath:    strings.TrimPrefix(strings.TrimPrefix(path, inputDir), string(os.PathSeparator)),
			ModTime:      info.ModTime(),
			CapturedTime: info.ModTime(),
		}

		// Extract EXIF dataif possible
		if fileExt == "jpg" || fileExt == "jpeg" || fileExt == "heic" || fileExt == "png" {
			fileInfo.ExifData = exifExtract(path)
			entry, found := fileInfo.ExifData["DateTimeOriginal"]
			if !found {
				entry, found = fileInfo.ExifData["DateTime"]
			}
			if !found {
				entry, found = fileInfo.ExifData["DateTimeDigitized"]
			}
			if found {
				if dateStr, ok := entry.Value.(string); ok {
					if capturedTime, err := time.Parse("2006:01:02 15:04:05", dateStr); err == nil {
						fileInfo.CapturedTime = capturedTime
						if false {
							fmt.Printf("%s %v\n", fileInfo.Name, fileInfo.CapturedTime)
						}
					}
				}
			}
		}

		// Append file information
		fileInfos = append(fileInfos, fileInfo)
		return nil
	})
	if err != nil {
		log.Fatalf("Error enumerating input directory: %v", err)
	}

	// Sort the FileInfo array by InputPath
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].InputPath < fileInfos[j].InputPath
	})

	// Process and copy files
	for _, fileInfo := range fileInfos {
		srcPath := filepath.Join(inputDir, fileInfo.InputPath)
		var destDir string
		var destFileName string

		// Check file type and decide destination directory
		switch strings.ToLower(fileInfo.Extension) {
		case "jpg", "jpeg", "heic", "mov", "mp4", "png", "gif":
			// Generate year-month subdirectory name based on CapturedTime
			yearMonth := fileInfo.CapturedTime.Format("2006-01")
			destDir = filepath.Join(outputDirRecognized, yearMonth)

			// Generate new filename in the desired format
			destFileName = fmt.Sprintf(
				"snap-%s-%s",
				fileInfo.CapturedTime.Format("2006-01-02-15-04-05"),
				strings.ToLower(strings.ReplaceAll(fileInfo.Name, "_", "")),
			)
		case "aae":
			continue
		default:
			destDir = outputDirError
			destFileName = fileInfo.Name // Keep original filename for unrecognized files
		}

		// Ensure the destination directory exists
		if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
			log.Printf("Error creating directory '%s': %v", destDir, err)
			continue
		}

		// Copy the file
		destPath := filepath.Join(destDir, destFileName)
		if err := copyFile(srcPath, destPath); err != nil {
			log.Printf("Error copying file '%s' to '%s': %v", srcPath, destPath, err)
		}
	}

}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open source file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("unable to create destination file: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("error during file copy: %v", err)
	}

	return nil
}
