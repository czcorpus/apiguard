// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package proxy

import "io"

type ReReader struct {
	data       []byte
	pos        int
	origReader io.ReadCloser
}

func NewReReader(r io.ReadCloser) (*ReReader, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &ReReader{origReader: r, data: data}, nil
}

func (rr *ReReader) Read(p []byte) (n int, err error) {
	if rr.pos >= len(rr.data) {
		rr.pos = 0 // reset for repeated reading
		return 0, io.EOF
	}

	n = copy(p, rr.data[rr.pos:])
	rr.pos += n
	return n, nil
}

func (rr *ReReader) Close() error {
	if rr.origReader != nil {
		return rr.origReader.Close()
	}
	return nil
}
