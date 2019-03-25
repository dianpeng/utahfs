package utahfs

import (
	"testing"

	"bytes"
	crand "crypto/rand"
	"io"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// memStorage implements the ObjectStorage interface over a map.
type memStorage map[string][]byte

func (ms memStorage) Get(key string) ([]byte, error) {
	d, ok := ms[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	data := make([]byte, len(d))
	copy(data, d)
	return data, nil
}

func (ms memStorage) Set(key string, data []byte) error {
	d := make([]byte, len(data))
	copy(d, data)
	ms[key] = d
	return nil
}

func (ms memStorage) Delete(key string) error {
	delete(ms, key)
	return nil
}

type testData struct {
	bf   *blockFile
	pos  int
	data []byte
}

func testBlockStorageSingle(t *testing.T, td *testData) {
	bf, pos, data := td.bf, td.pos, td.data

	dice := rand.Intn(30)

	if len(data) > 0 && dice == 0 { // Seek from start.
		off := rand.Int63n(int64(len(data)))
		got, err := bf.Seek(off, io.SeekStart)
		if err != nil {
			t.Fatal(err)
		}
		pos = int(off)
		if got != int64(pos) {
			t.Fatalf("%v != %v", got, pos)
		}
	} else if len(data) > 0 && pos != len(data) && dice == 1 { // Seek from current.
		off := rand.Int63n(int64(len(data) - pos))
		got, err := bf.Seek(off, io.SeekCurrent)
		if err != nil {
			t.Fatal(err)
		}
		pos += int(off)
		if got != int64(pos) {
			t.Fatalf("%v != %v", got, pos)
		}
	} else if len(data) > 0 && dice == 2 { // Seek from end.
		off := rand.Int63n(int64(len(data)))
		got, err := bf.Seek(off, io.SeekEnd)
		if err != nil {
			t.Fatal(err)
		}
		pos = len(data) - int(off)
		if got != int64(pos) {
			t.Fatalf("%v != %v", got, pos)
		}
	} else if len(data) > 0 && dice < 20 { // Read.
		p := make([]byte, rand.Int63n(dataSize)+1)

		n, err := bf.Read(p)
		if pos == len(data) {
			if err != io.EOF {
				t.Fatal("expected EOF when reading at end of file")
			} else if n != 0 {
				t.Fatal("read past end of file")
			}
		} else if pos != len(data) {
			if err != nil {
				t.Fatal(err)
			} else if n == 0 {
				t.Fatal("read zero bytes even though there was data")
			}
		}
		if n > len(p) || pos+n > len(data) {
			t.Fatalf("%v violates bounds", n)
		} else if !bytes.Equal(data[pos:pos+n], p[:n]) {
			t.Fatal("read unexpected data")
		}
		pos += n
	} else { // Write.
		p := make([]byte, rand.Int63n(3*dataSize)+1)
		crand.Read(p)

		n, err := bf.Write(p)
		if err != nil {
			t.Fatal(err)
		} else if n != len(p) {
			t.Fatalf("%v != %v", n, len(p))
		}

		if pos+n > len(data) {
			data = append(data[:pos], p...)
		} else {
			copy(data[pos:pos+n], p)
		}
		pos += n
	}

	td.pos, td.data = pos, data
}

func TestBlockStorage(t *testing.T) {
	bs := &blockStorage{
		store: make(memStorage),
		trash: nilPtr,
		next:  0,
	}

	ptrs := make([]uint32, 0)
	files := make(map[uint32]*testData)

	for i := 0; i < 10000; i++ {
		dice := rand.Intn(1000)

		if len(ptrs) == 0 || dice == 0 {
			ptr, bf, err := bs.Create()
			if err != nil {
				t.Fatal(err)
			}
			ptrs = append(ptrs, ptr)
			files[ptr] = &testData{bf, 0, nil}
		} else {
			ptr := ptrs[rand.Intn(len(ptrs))]
			testBlockStorageSingle(t, files[ptr])
		}
	}

	t.Logf("created %v files", len(ptrs))
	sum := 0
	for _, ptr := range ptrs {
		n := len(files[ptr].data)
		sum += n
		t.Logf("- %x: contains %v bytes", ptr, n)
	}
	t.Logf("%v bytes total", sum)
}
