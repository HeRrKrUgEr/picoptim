package controllers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chai2010/webp"
	"github.com/gin-gonic/gin"
)

func OptimizeImage(c *gin.Context) {
	uploadsRootDir := "./uploads"
	downloadsRootDir := "./downloads"
	// Ensure the Uploads directory exists
	if err := os.MkdirAll(uploadsRootDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create uploads root directory: %v", err)})
		return
	}

	if err := os.MkdirAll(downloadsRootDir, os.ModePerm); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create Downloads root directory: %v", err)})
		return
	}

	// Create a temporary directory inside the Uploads directory
	tempDir, err := os.MkdirTemp(uploadsRootDir, "upload_*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create temp directory: %v", err)})
		return
	}
	tempOutDir, err := os.MkdirTemp(downloadsRootDir, "download_*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create tempOut directory: %v", err)})
		return
	}

	form, _ := c.MultipartForm()
	files := filterFiles(form.File["images"], func(f *multipart.FileHeader) bool {
		return strings.HasSuffix(strings.ToLower(f.Filename), ".jpg") || strings.HasSuffix(strings.ToLower(f.Filename), ".jpeg") || strings.HasSuffix(strings.ToLower(f.Filename), ".webp") || strings.HasSuffix(strings.ToLower(f.Filename), ".heic") || strings.HasSuffix(strings.ToLower(f.Filename), ".png") || strings.HasSuffix(strings.ToLower(f.Filename), ".zip")
	})
	useCase := c.PostForm("use_case")
	outputFormat := c.PostForm("output_format")
	qualityStr := c.PostForm("quality")
	quality, err := strconv.Atoi(qualityStr)
	if err != nil || quality < 1 || quality > 100 {
		quality = 80 // default quality if parsing fails or invalid value provided
	}

	if useCase == "" || outputFormat == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use case and output format are required"})
		return
	}

	var targetWidth, targetHeight int
	switch useCase {
	case "keep":
		targetWidth, targetHeight = 0, 0
	case "hero":
		targetWidth, targetHeight = 1920, 1080
	case "page":
		targetWidth, targetHeight = 1200, 800
	case "prop":
		targetWidth, targetHeight = 640, 480
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid use case"})
		return
	}

	var optimizedFiles []ImageData
	for i := 0; i < len(files); i++ {
		file := files[i]
		if strings.HasSuffix(file.Filename, ".zip") {
			zipPath := filepath.Join(tempDir, file.Filename)
			if err := c.SaveUploadedFile(file, zipPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save ZIP file: %v", err)})
				return
			}
			if zipFiles, err := unzipFiles(zipPath, tempDir); err == nil {
				files = append(files[:i], filterFiles(zipFiles, func(f *multipart.FileHeader) bool {
					return strings.HasSuffix(strings.ToLower(f.Filename), "jpg") || strings.HasSuffix(strings.ToLower(f.Filename), "jpeg") || strings.HasSuffix(strings.ToLower(f.Filename), "webp") || strings.HasSuffix(strings.ToLower(f.Filename), "heic") || strings.HasSuffix(strings.ToLower(f.Filename), "png")
				})...)
				files = append(files, files[i+1:]...)
				i--
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to unzip file: %v", err)})
				return
			}
		} else {
			tempFilePath := filepath.Join(tempDir, file.Filename)
			if err := c.SaveUploadedFile(file, tempFilePath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save uploaded file: %v", err)})
				return
			}
			originalSize := file.Size
			outputFileName := fmt.Sprintf("%s.%s", strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename)), outputFormat)
			outputPath := filepath.Join(tempOutDir, outputFileName)

			// Handle HEIC files separately using ImageMagick to convert to JPEG first
			if strings.HasSuffix(strings.ToLower(file.Filename), ".heic") {
				heicOutputPath := strings.Replace(strings.ToLower(tempFilePath), ".heic", ".png", 1)
				cmd := exec.Command("magick", tempFilePath, heicOutputPath)
				if err := cmd.Run(); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to convert HEIC file: %v", err)})
					return
				}
				tempFilePath = heicOutputPath
			}

			// Use ImageMagick to handle conversion, resizing, and orientation correction
			cmdArgs := []string{filepath.Join(tempFilePath)}

			// If resizing is needed
			if targetWidth > 0 || targetHeight > 0 {
				resizeArg := fmt.Sprintf("%dx%d", targetWidth, targetHeight)
				cmdArgs = append(cmdArgs, "-resize", resizeArg)
			}

			// Correct the orientation based on EXIF data
			cmdArgs = append(cmdArgs, "-auto-orient")

			// Set output quality for JPEG or WebP
			if outputFormat == "jpg" || outputFormat == "webp" {
				qualityArg := fmt.Sprintf("%d", quality)
				cmdArgs = append(cmdArgs, "-quality", qualityArg)
			}

			// Set the output format
			cmdArgs = append(cmdArgs, outputPath)
			cmd := exec.Command("magick", cmdArgs...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process image with ImageMagick: %v", stderr.String())})
				return
			}

			outputFile, err := os.Open(outputPath)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open optimized file: %v", err)})
				return
			}

			// Get optimized file size
			optimizedFileInfo, err := outputFile.Stat()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get optimized file info: %v", err)})
				return
			}
			optimizedSize := optimizedFileInfo.Size()
			defer outputFile.Close()
			imageData := ImageData{
				Name:          outputFileName,
				OriginalSize:  originalSize,
				OptimizedSize: optimizedSize,
				Reduction:     int(((float64(originalSize) - float64(optimizedSize)) / float64(originalSize)) * 100),
				Preview:       filepath.Join(tempOutDir, filepath.Base(outputPath)),
				DownloadUrl:   filepath.Join(tempOutDir, filepath.Base(outputPath)),
			}
			// Add optimized file details to response
			optimizedFiles = append(optimizedFiles, imageData)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"uidPath":          tempOutDir,
		"optimized_images": optimizedFiles,
	})
}

type ImageData struct {
	Name          string `json:"name"`
	OriginalSize  int64  `json:"originalSize"`
	OptimizedSize int64  `json:"optimizedSize"`
	Reduction     int    `json:"reduction"`
	Preview       string
	DownloadUrl   string
}

func encodeWebP(outputFile *os.File, img image.Image, quality int) error {
	return webp.Encode(outputFile, img, &webp.Options{Quality: float32(quality)})
}

func DownloadZip(c *gin.Context) {

	outputDir := c.PostForm("outputDir")
	zipFilePath := filepath.Join(outputDir, "/optimized_images.zip")

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		fmt.Println("Erreur lors de la lecture du dossier :", err)
		return
	}

	var filePaths []string
	for _, fileOb := range entries {
		filePaths = append(filePaths, fmt.Sprintf("%s/%s", outputDir, fileOb.Name()))
	}

	if err := zipFiles(filePaths, zipFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create ZIP file: %v", err)})
		return
	}
	data := gin.H{
		"zip_download_url": zipFilePath,
	}

	// Retourner la réponse JSON
	c.JSON(200, data)

}

func zipFiles(files []string, output string) error {
	zipFile, err := os.Create(output)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		w, err := zipWriter.Create(filepath.Base(file))
		if err != nil {
			return err
		}

		if _, err := io.Copy(w, f); err != nil {
			return err
		}
	}

	return nil
}

func unzipFiles(zipPath, destDir string) ([]*multipart.FileHeader, error) {
	var extractedFiles []*multipart.FileHeader
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer zipReader.Close()

	for _, f := range zipReader.File {
		if !strings.HasSuffix(f.Name, ".jpg") && !strings.HasSuffix(f.Name, ".jpeg") && !strings.HasSuffix(f.Name, ".png") {
			continue
		}
		fPath := filepath.Join(destDir, f.Name)
		if err := os.MkdirAll(filepath.Dir(fPath), os.ModePerm); err != nil {
			return nil, err
		}

		outFile, err := os.Create(fPath)
		if err != nil {
			return nil, err
		}
		defer outFile.Close()

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		if _, err := io.Copy(outFile, rc); err != nil {
			return nil, err
		}
	}

	return extractedFiles, nil
}

func filterFiles(slice []*multipart.FileHeader, predicate func(*multipart.FileHeader) bool) []*multipart.FileHeader {
	var result []*multipart.FileHeader
	for _, value := range slice {
		if predicate(value) {
			result = append(result, value)
		}
	}
	return result
}
