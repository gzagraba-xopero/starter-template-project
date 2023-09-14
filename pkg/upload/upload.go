package upload

type Upload interface {
	UploadFrom(path string) error
	Shutdown() error
}
