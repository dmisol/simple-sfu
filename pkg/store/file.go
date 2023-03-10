package store

import (
	"log"
	"os"
	"path"
	"sync"
	//	"syscall"

	"github.com/dmisol/simple-sfu/pkg/defs"
	"github.com/valyala/fasthttp"
)

type FileStore struct {
	mu   sync.Mutex
	conf *defs.Conf
}

func NewFileStore(c *defs.Conf) (x *FileStore) {
	x = &FileStore{
		conf: c,
	}
	err := os.MkdirAll(c.Folder, 0777)
	if err != nil {
		log.Fatalln("storage folder not created", c.Folder, err)
	}
	return
}

func (x *FileStore) Handler(r *fasthttp.RequestCtx) {
	qa := r.QueryArgs()
	filename := string(qa.Peek("f"))
	if filename == "" {
		r.Error("invalid request", fasthttp.StatusBadRequest)
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()

	if string(r.Method()) == "GET" {
		log.Println("downloading file", filename)
		r.SendFile(path.Join(x.conf.Folder, filename))
		return
	}

	if string(r.Method()) == "POST" {
		log.Println("uploading file", filename)
		err := os.WriteFile(path.Join(x.conf.Folder, filename), r.PostBody(), 0777)
		if err != nil {
			log.Println("error uploading file", filename, err)
			r.Error("error uploading file", fasthttp.StatusInternalServerError)
			return
		}
		log.Println("uploaded file", filename, err)
		return
	}

	r.Error("method not supported", fasthttp.StatusMethodNotAllowed)
}
