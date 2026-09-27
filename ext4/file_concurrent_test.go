package ext4

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func TestFileRead_Concurrent(t *testing.T) {
	// Two Files opened from the same FileSystem (sharing the same reader) point
	// to different block ranges. Reading them concurrently must not mix up the
	// data, which happens if Read depends on the reader's shared offset.
	const blockSize = 1024
	const blocksPerFile = 64
	const fileSize = blockSize * blocksPerFile

	// Every block in the image has a distinct byte value, so reading a wrong
	// block is always detected.
	image := make([]byte, blockSize*blocksPerFile*2)
	for i := 0; i < blocksPerFile*2; i++ {
		for j := 0; j < blockSize; j++ {
			image[i*blockSize+j] = byte(i)
		}
	}

	table1 := dataTable{}
	table2 := dataTable{}
	for i := int64(0); i < blocksPerFile; i++ {
		table1[i] = i * blockSize
		table2[i] = (i + blocksPerFile) * blockSize
	}

	f1 := newTestFile(image, blockSize, fileSize, table1)
	f2 := newTestFile(image, blockSize, fileSize, table2)
	f2.fs = f1.fs

	// t.Fatal must not be called from other goroutines, so errors are
	// collected here and checked after all reads finish.
	var wg sync.WaitGroup
	results := make([][]byte, 2)
	errs := make([]error, 2)
	for i, f := range []*File{f1, f2} {
		wg.Add(1)
		go func(i int, f *File) {
			defer wg.Done()
			results[i], errs[i] = io.ReadAll(f)
		}(i, f)
	}
	wg.Wait()

	want := [][]byte{
		image[:fileSize],
		image[fileSize:],
	}
	for i := range results {
		if errs[i] != nil {
			t.Fatalf("file %d: Read error: %v", i+1, errs[i])
		}
		if !bytes.Equal(results[i], want[i]) {
			t.Errorf("file %d: read data does not match the expected block range", i+1)
		}
	}
}
