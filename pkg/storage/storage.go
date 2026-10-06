package storage

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
)

// ObjectStorage represents an object storage to store file object
type ObjectStorage interface {
	Exists(ctx core.Context, path string) (bool, error)
	Read(ctx core.Context, path string) (object ObjectInStorage, exists bool, err error)
	Save(ctx core.Context, path string, object ObjectInStorage) error
	Delete(ctx core.Context, path string) error
}

// ObjectMeta represents the metadata of a file object in the storage
type ObjectMeta struct {
	// Path is the object path relative to the root path of the storage, and the path separator is always slash
	Path string
	// Size is the size in bytes of the object
	Size int64
	// LastModified is the last modification time of the object
	LastModified time.Time
}

// ListableObjectStorage represents an object storage which supports listing objects under specified path prefix
type ListableObjectStorage interface {
	// List returns metadata of all objects whose path starts with the specified prefix.
	// The returned path is relative to the storage root path and uses slash as the separator regardless of the actual storage type.
	// The implementation must fail closed, that means it must return an error instead of a partial result when the listing cannot be guaranteed to be complete.
	List(ctx core.Context, prefix string) ([]ObjectMeta, error)
}
