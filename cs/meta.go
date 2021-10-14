package cs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"strings"
)

func validateMeta(b []byte) error {
	var (
		seenTrailer, seenMeta bool
		meta                  = map[string]string{}
		metaPrefix, trailer   = []byte("<</Creator"), []byte("trailer")
	)
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.HasPrefix(line, trailer) {
			if seenTrailer {
				return errors.New("duplicate trailers")
			}
			seenTrailer = true
		}
		if bytes.HasPrefix(line, metaPrefix) {
			if seenMeta {
				return errors.New("duplicate meta")
			}
			seenMeta = true
			metaFields := strings.FieldsFunc(string(line[2:]), func(r rune) bool { return r == '/' })
			for _, metaKV := range metaFields {
				parts := strings.SplitN(metaKV, "(", 2)
				if len(parts) != 2 {
					continue // some leftover value?
				}
				meta[parts[0]] = parts[1]
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w", err)
	}
	switch {
	case !strings.HasPrefix(meta["Creator"], "Computershare Communication Services, GPD 3.00"):
		log.Printf("unusual invalid creator %v", meta["Creator"])
		return errors.New("invalid creator")
	case !strings.HasPrefix(meta["Producer"], "PDFlib+PDI 7.0.4p1"):
		log.Printf("unusual invalid producer %v", meta["Producer"])
		return errors.New("invalid producer")
	case strings.TrimSpace(meta["Subject"]) == "":
		log.Printf("unusual empty subject %v", meta["Subject"])
		return errors.New("invalid subject")
	}
	return nil
}
