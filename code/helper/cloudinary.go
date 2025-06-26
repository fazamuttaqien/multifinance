package helper

import (
	"context"
	"mime/multipart"
	"os"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
)

type cloudinaryUploader struct {
	cld *cloudinary.Cloudinary
}

func NewCloudinaryUploader() (*cloudinaryUploader, error) {
	cld, err := cloudinary.NewFromURL(os.Getenv("CLOUDINARY_URL"))
	if err != nil {
		return nil, err
	}

	return &cloudinaryUploader{cld: cld}, nil
}

func (cu *cloudinaryUploader) Upload(ctx context.Context, fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	uploadParams := uploader.UploadParams{
		Folder: os.Getenv("CLOUDINARY_UPLOAD_FOLDER"),
	}

	result, err := cu.cld.Upload.Upload(ctx, file, uploadParams)
	if err != nil {
		return "", err
	}

	return result.SecureURL, nil
}
