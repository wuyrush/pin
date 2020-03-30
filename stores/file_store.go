package stores

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"

	cst "wuyrush.io/pin/constants"
	pe "wuyrush.io/pin/errors"
)

// FileStore stores attachment files of arbitrary type associated with a given pin
// (note a file is just a byte sequence)
type FileStore interface {
	// Ref returns the reference of file in file storage layer for future persistence and access. It should
	// always be deterministic based on pin ID and filename
	Ref(pinID, filename string) string
	Save(ref string, r io.ReadCloser) *pe.PinErr
	Get(ref string) (io.ReadCloser, *pe.PinErr)
	// Delete deletes pin attachments from store. Delete must be idempotent
	Delete(ref string) *pe.PinErr
	Close() *pe.PinErr
}

// LocalFileStore implements FileStore backed by local file system
type LocalFileStore struct {
}

func (fs *LocalFileStore) Ref(pinID, filename string) string {
	// TODO: this doesn't scale under high write traffic due to inode exhausation. Essentially local fs storage solution won't scale at all;
	// leveraging third-party services like S3 if pins with attachments are really growing
	return filepath.Join(string(filepath.Separator), "tmp", pinID, filename)
}

func (fs *LocalFileStore) Save(ref string, r io.ReadCloser) *pe.PinErr {
	pinAttachmentMaxSizeByte := viper.GetInt64(cst.EnvPinAttachmentSizeMaxByte)
	// 1. prepare file to host data
	errMsg := "error allocating file storage space"
	dir := filepath.Dir(ref)
	if err := os.MkdirAll(dir, os.ModeDir); err != nil {
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	f, err := os.Create(ref)
	defer f.Close()
	if err != nil {
		return pe.ErrServiceFailure(errMsg).WithCause(err)
	}
	// 2. pipe data to file
	br := bufio.NewReader(http.MaxBytesReader(nil, r, pinAttachmentMaxSizeByte))
	if _, err := br.WriteTo(f); err != nil {
		if strings.Index(err.Error(), cst.ErrMsgRequestBodyTooLarge) >= 0 {
			return pe.ErrBadInput("pin attachment oversized").WithCause(err)
		}
		return pe.ErrServiceFailure("error saving pin attachment data").WithCause(err)
	}
	return nil
}

func (fs *LocalFileStore) Get(ref string) (io.ReadCloser, *pe.PinErr) {
	f, err := os.Open(ref)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, pe.ErrNotFound("pin attachment not found").WithCause(err)
		}
		return nil, pe.ErrServiceFailure("error retriving pin attachment")
	}
	return f, nil
}

func (fs *LocalFileStore) Delete(ref string) *pe.PinErr {
	if err := os.Remove(ref); err != nil && !os.IsNotExist(err) {
		return pe.ErrServiceFailure("error removing pin attachment").WithCause(err)
	}
	return nil
}

func (fs *LocalFileStore) Close() *pe.PinErr {
	return nil
}
