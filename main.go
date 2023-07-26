package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/kopia/kopia/fs"
	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/blob/filesystem"
	"github.com/kopia/kopia/repo/object"
	"github.com/kopia/kopia/snapshot"
)

func main() {
	n := time.Now()
	dir, err := os.MkdirTemp("", "test-kopia")
	if err != nil {
		panic(err)
	}

	storage, err := filesystem.New(context.Background(), &filesystem.Options{Path: dir}, true)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", storage)

	err = repo.Initialize(context.Background(), storage, nil, "test")
	if err != nil {
		panic(err)
	}

	err = repo.Connect(context.Background(), "file.config", storage, "test", nil)
	if err != nil {
		panic(err)
	}

	r, err := repo.Open(context.Background(), "file.config", "test", nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v", r)

	ctx, w, err := r.NewWriter(context.Background(), repo.WriteSessionOptions{})

	fmt.Printf("%#v - %#v = %#v", ctx, w, err)

	// hard code the block device
	dat, err := os.Open("/dev/loop0")
	if err != nil {
		panic(err)
	}

	ch := make(chan []byte, 5)

	ids := []object.ID{}
	go func() {
		for {
			select {
			case b := <-ch:
				wr := w.NewObjectWriter(context.Background(), object.WriterOptions{})
				_, err := wr.Write(b)
				if err != nil {
					panic(err)
				}
				id, err := wr.Result()
				if err != nil {
					panic(err)
				}
				ids = append(ids, id)
				fmt.Printf("\n%#v\n", id)

			}
		}
	}()

	// 100 Megabyte
	whileFile := true
	for whileFile {
		b1 := make([]byte, 100000000)
		_, err := dat.Read(b1)
		if err != nil {
			if err == io.EOF {
				fmt.Printf("here")
				whileFile = false
			} else {
				panic(err)
			}
		}
		ch <- b1
	}

	id, err := w.ConcatenateObjects(context.Background(), ids)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%#v", id.String())

	manifest := snapshot.Manifest{
		Source: snapshot.SourceInfo{
			Host:     "localhost",
			Path:     "/dev/loop0",
			UserName: "shawn",
		},
		StartTime:   fs.UTCTimestampFromTime(n),
		EndTime:     fs.UTCTimestampFromTime(time.Now()),
		Description: "Description",
		RootEntry: &snapshot.DirEntry{
			Name:        "/dev/loop0",
			Type:        snapshot.EntryTypeFile,
			Permissions: snapshot.Permissions(777),
			FileSize:    30000000,
			UserID:      1001,
			GroupID:     1001,
			ObjectID:    id,
			ModTime:     fs.UTCTimestampFromTime(time.Now()),
			DirSummary: &fs.DirectorySummary{
				TotalFileCount: 1,
				TotalFileSize:  30000000,
				MaxModTime:     fs.UTCTimestampFromTime(time.Now()),
			},
		},
	}
	mid, err := snapshot.SaveSnapshot(ctx, w, &manifest)
	if err != nil {
		panic(err)
	}
	fmt.Printf("here: %v", mid)

	err = w.Flush(context.Background())
	if err != nil {
		panic(err)
	}

	time.Sleep(1 * time.Minute)

	err = r.Refresh(context.Background())
	if err != nil {
		panic(err)
	}
	s, err := snapshot.LoadSnapshot(context.Background(), r, mid)
	if err != nil {
		panic(err)
	}
	fmt.Printf("\n%v\n", s)
}
