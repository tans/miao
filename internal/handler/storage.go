package handler

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/storage"
)

var (
	storageProvider storage.StorageProvider
	storageOnce     sync.Once
	storageErr      error
	storageWorkDir  string
)

func initStorage() error {
	storageOnce.Do(func() {
		// Determine work directory
		storageWorkDir, _ = filepath.Abs(filepath.Dir(os.Args[0]))
		if storageWorkDir == "" || storageWorkDir == "." {
			storageWorkDir, _ = os.Getwd()
		}

		cfg := config.Load()

		factory := storage.NewFactory(cfg.Static.Host, cfg.Static.CDN, storageWorkDir)

		var cfgType storage.StorageType
		switch cfg.Storage.Provider {
		case "rustfs":
			cfgType = storage.StorageTypeRustFS
		case "s3":
			cfgType = storage.StorageTypeS3
		case "oss":
			cfgType = storage.StorageTypeOSS
		case "cos":
			cfgType = storage.StorageTypeCOS
		default:
			cfgType = storage.StorageTypeLocal
		}

		storageCfg := storage.Config{
			Type: cfgType,
			Local: storage.LocalConfig{
				BaseDir: "",
				BaseURL: "",
			},
			RustFS: storage.RustFSConfig{
				Endpoint:  cfg.Storage.RustFS.Endpoint,
				Bucket:    cfg.Storage.RustFS.Bucket,
				AccessKey: cfg.Storage.RustFS.AccessKey,
				SecretKey: cfg.Storage.RustFS.SecretKey,
				Region:    cfg.Storage.RustFS.Region,
			},
			S3: storage.S3Config{
				Endpoint:        cfg.Storage.S3.Endpoint,
				Bucket:          cfg.Storage.S3.Bucket,
				Region:          cfg.Storage.S3.Region,
				AccessKeyID:     cfg.Storage.S3.AccessKeyID,
				SecretAccessKey: cfg.Storage.S3.SecretAccessKey,
			},
			OSS: storage.OSSConfig{
				Endpoint:    cfg.Storage.OSS.Endpoint,
				Bucket:      cfg.Storage.OSS.Bucket,
				AccessKeyID: cfg.Storage.OSS.AccessKey,
				SecretKey:   cfg.Storage.OSS.SecretKey,
				CDNHost:     cfg.Storage.OSS.CDNHost,
			},
			COS: storage.COSConfig{
				AppID:     cfg.Storage.COS.AppID,
				Bucket:    cfg.Storage.COS.Bucket,
				Region:    cfg.Storage.COS.Region,
				SecretKey: cfg.Storage.COS.SecretKey,
				SecretID:  cfg.Storage.COS.SecretID,
				CDNHost:   cfg.Storage.COS.CDNHost,
			},
		}

		storageProvider, storageErr = factory.NewProvider(storageCfg)
	})
	return storageErr
}

// GetStorageProvider returns the current storage provider.
// It lazily initializes storage on first call.
func GetStorageProvider() (storage.StorageProvider, error) {
	if storageProvider == nil {
		if err := initStorage(); err != nil {
			return nil, err
		}
	}
	return storageProvider, nil
}
