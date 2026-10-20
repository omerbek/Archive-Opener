package arc

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/flate"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz/lzma"
	"golang.org/x/crypto/pbkdf2"
)

func init() {
	zip.RegisterDecompressor(12, func(r io.Reader) io.ReadCloser { return io.NopCloser(bzip2.NewReader(r)) })
	zip.RegisterDecompressor(14, newZipLZMA)
	zip.RegisterDecompressor(93, func(r io.Reader) io.ReadCloser {
		d, err := zstd.NewReader(r)
		if err != nil {
			return io.NopCloser(&errReader{err})
		}
		return d.IOReadCloser()
	})
}

type errReader struct{ err error }

func (e *errReader) Read([]byte) (int, error) { return 0, e.err }

// newZipLZMA reads method 14: 2-byte version, 2-byte props size, props, then raw LZMA.
func newZipLZMA(r io.Reader) io.ReadCloser {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return io.NopCloser(&errReader{err})
	}
	psize := int(binary.LittleEndian.Uint16(hdr[2:]))
	props := make([]byte, psize)
	if _, err := io.ReadFull(r, props); err != nil || psize < 5 {
		return io.NopCloser(&errReader{errors.New("zip: bad lzma header")})
	}
	// Build a classic .lzma header: props(5) + unknown size (8 x 0xff).
	h := append(props[:5:5], 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff)
	lr, err := lzma.NewReader(io.MultiReader(bytes.NewReader(h), r))
	if err != nil {
		return io.NopCloser(&errReader{err})
	}
	return io.NopCloser(lr)
}

type zipArchive struct {
	f        *os.File
	zr       *zip.Reader
	password string
	entries  []*Entry
	files    []*zip.File
}

func openZIP(path, password string) (Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	a := &zipArchive{f: f, zr: zr, password: password}
	for i, zf := range zr.File {
		name := zf.Name
		if zf.Flags&0x800 == 0 && !utf8.ValidString(name) {
			// Old Windows tools store names in the OEM code page; for
			// Turkish systems that is CP857.
			name = decodeCP857(name)
		}
		name = strings.TrimLeft(strings.ReplaceAll(name, "\\", "/"), "/")
		mode := zf.Mode()
		e := &Entry{
			Path:      name,
			Size:      int64(zf.UncompressedSize64),
			Packed:    int64(zf.CompressedSize64),
			Modified:  zf.Modified,
			IsDir:     strings.HasSuffix(name, "/") || mode.IsDir(),
			Encrypted: zf.Flags&1 != 0,
			IsLink:    mode&os.ModeSymlink != 0,
			index:     i,
		}
		if e.IsDir && !strings.HasSuffix(e.Path, "/") {
			e.Path += "/"
		}
		a.entries = append(a.entries, e)
		a.files = append(a.files, zf)
	}
	return a, nil
}

func (a *zipArchive) Format() string    { return "ZIP" }
func (a *zipArchive) Entries() []*Entry { return a.entries }
func (a *zipArchive) Close() error      { return a.f.Close() }

func (a *zipArchive) Walk(want func(*Entry) bool, fn func(*Entry, io.Reader) error) error {
	for i, e := range a.entries {
		if want != nil && !want(e) {
			continue
		}
		if e.IsDir || e.IsLink {
			if err := fn(e, strings.NewReader("")); err != nil {
				return err
			}
			continue
		}
		rc, err := a.open(a.files[i], e)
		if err != nil {
			return err
		}
		err = fn(e, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *zipArchive) open(zf *zip.File, e *Entry) (io.ReadCloser, error) {
	if !e.Encrypted {
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		return rc, nil
	}
	if a.password == "" {
		return nil, ErrNeedPassword
	}
	raw, err := zf.OpenRaw()
	if err != nil {
		return nil, err
	}
	if zf.Method == 99 {
		return openWinZipAES(zf, raw, a.password)
	}
	return openZipCrypto(zf, raw, a.password)
}

func decompressor(method uint16, r io.Reader) (io.ReadCloser, error) {
	switch method {
	case zip.Store:
		return io.NopCloser(r), nil
	case zip.Deflate:
		return flate.NewReader(r), nil
	case 12:
		return io.NopCloser(bzip2.NewReader(r)), nil
	case 14:
		return newZipLZMA(r), nil
	case 93:
		d, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return d.IOReadCloser(), nil
	}
	return nil, errors.New("zip: desteklenmeyen sıkıştırma yöntemi")
}

/* ---------- traditional PKWARE encryption (ZipCrypto) ---------- */

type zipCryptoKeys [3]uint32

func (k *zipCryptoKeys) update(c byte) {
	k[0] = crc32.IEEETable[byte(k[0])^c] ^ (k[0] >> 8)
	k[1] = (k[1]+(k[0]&0xff))*134775813 + 1
	k[2] = crc32.IEEETable[byte(k[2])^byte(k[1]>>24)] ^ (k[2] >> 8)
}

func (k *zipCryptoKeys) stream() byte {
	t := uint16(k[2] | 2)
	return byte((uint32(t) * uint32(t^1)) >> 8)
}

type zipCryptoReader struct {
	r    io.Reader
	keys zipCryptoKeys
}

func (z *zipCryptoReader) Read(p []byte) (int, error) {
	n, err := z.r.Read(p)
	for i := 0; i < n; i++ {
		c := p[i] ^ z.keys.stream()
		z.keys.update(c)
		p[i] = c
	}
	return n, err
}

func openZipCrypto(zf *zip.File, raw io.Reader, password string) (io.ReadCloser, error) {
	keys := zipCryptoKeys{0x12345678, 0x23456789, 0x34567890}
	for i := 0; i < len(password); i++ {
		keys.update(password[i])
	}
	zc := &zipCryptoReader{r: raw, keys: keys}
	var hdr [12]byte
	if _, err := io.ReadFull(zc, hdr[:]); err != nil {
		return nil, err
	}
	check := byte(zf.CRC32 >> 24)
	if zf.Flags&0x8 != 0 {
		check = byte(zf.ModifiedTime >> 8)
	}
	if hdr[11] != check {
		return nil, ErrBadPassword
	}
	dr, err := decompressor(zf.Method, zc)
	if err != nil {
		return nil, err
	}
	return &crcCheckReader{rc: dr, want: zf.CRC32, h: crc32.NewIEEE(), size: zf.UncompressedSize64}, nil
}

// crcCheckReader verifies the CRC at EOF; a mismatch on an encrypted entry
// almost always means the password was wrong.
type crcCheckReader struct {
	rc   io.ReadCloser
	want uint32
	h    hash.Hash32
	size uint64
	n    uint64
}

func (c *crcCheckReader) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	c.h.Write(p[:n])
	c.n += uint64(n)
	if err == io.EOF && c.h.Sum32() != c.want {
		return n, ErrBadPassword
	}
	if err != nil && err != io.EOF {
		return n, ErrBadPassword
	}
	return n, err
}

func (c *crcCheckReader) Close() error { return c.rc.Close() }

/* ---------- WinZip AES (AE-1 / AE-2) ---------- */

func openWinZipAES(zf *zip.File, raw io.Reader, password string) (io.ReadCloser, error) {
	strength, method, ok := parseAESExtra(zf.Extra)
	if !ok {
		return nil, errors.New("zip: AES bilgisi bulunamadı")
	}
	keyLen := map[byte]int{1: 16, 2: 24, 3: 32}[strength]
	if keyLen == 0 {
		return nil, errors.New("zip: bilinmeyen AES gücü")
	}
	saltLen := keyLen / 2
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(raw, salt); err != nil {
		return nil, err
	}
	var pv [2]byte
	if _, err := io.ReadFull(raw, pv[:]); err != nil {
		return nil, err
	}
	dk := pbkdf2.Key([]byte(password), salt, 1000, 2*keyLen+2, sha1.New)
	if subtle.ConstantTimeCompare(dk[2*keyLen:], pv[:]) != 1 {
		return nil, ErrBadPassword
	}
	dataLen := int64(zf.CompressedSize64) - int64(saltLen) - 2 - 10
	if dataLen < 0 {
		return nil, errors.New("zip: bozuk AES verisi")
	}
	block, err := aes.NewCipher(dk[:keyLen])
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha1.New, dk[keyLen:2*keyLen])
	ar := &aesCTRReader{
		r:     io.LimitReader(raw, dataLen),
		raw:   raw,
		block: block,
		mac:   mac,
	}
	dr, err := decompressor(method, ar)
	if err != nil {
		return nil, err
	}
	return dr, nil
}

func parseAESExtra(extra []byte) (strength byte, method uint16, ok bool) {
	for len(extra) >= 4 {
		id := binary.LittleEndian.Uint16(extra)
		size := int(binary.LittleEndian.Uint16(extra[2:]))
		if len(extra) < 4+size {
			return
		}
		if id == 0x9901 && size >= 7 {
			d := extra[4:]
			return d[4], binary.LittleEndian.Uint16(d[5:]), true
		}
		extra = extra[4+size:]
	}
	return
}

// aesCTRReader decrypts WinZip's AES-CTR (little-endian counter from 1) and
// checks the 10-byte HMAC-SHA1 trailer at the end.
type aesCTRReader struct {
	r       io.Reader
	raw     io.Reader
	block   cipher.Block
	mac     hash.Hash
	counter [16]byte
	ks      [16]byte
	ksPos   int
	started bool
	done    bool
}

func (a *aesCTRReader) Read(p []byte) (int, error) {
	if a.done {
		return 0, io.EOF
	}
	if !a.started {
		a.started = true
		a.ksPos = 16
	}
	n, err := a.r.Read(p)
	a.mac.Write(p[:n])
	for i := 0; i < n; i++ {
		if a.ksPos == 16 {
			for j := 0; j < 16; j++ {
				a.counter[j]++
				if a.counter[j] != 0 {
					break
				}
			}
			a.block.Encrypt(a.ks[:], a.counter[:])
			a.ksPos = 0
		}
		p[i] ^= a.ks[a.ksPos]
		a.ksPos++
	}
	if err == io.EOF {
		a.done = true
		var tag [10]byte
		if _, terr := io.ReadFull(a.raw, tag[:]); terr != nil {
			return n, terr
		}
		if !hmac.Equal(a.mac.Sum(nil)[:10], tag[:]) {
			return n, ErrBadPassword
		}
	}
	return n, err
}
